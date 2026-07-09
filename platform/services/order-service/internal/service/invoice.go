package service

import (
	"context"
	"fmt"
	"time"

	"github.com/zippyra/platform/services/order-service/internal/model"
	"github.com/zippyra/platform/services/order-service/internal/repository"
)

type KafkaPublisher interface {
	Publish(ctx context.Context, topic, key string, value interface{}) error
}

type InvoiceService struct {
	orderRepo     *repository.OrderRepository
	orderItemRepo *repository.OrderItemRepository
	producer      KafkaPublisher
}

func NewInvoiceService(
	orderRepo *repository.OrderRepository,
	orderItemRepo *repository.OrderItemRepository,
	producer KafkaPublisher,
) *InvoiceService {
	return &InvoiceService{
		orderRepo:     orderRepo,
		orderItemRepo: orderItemRepo,
		producer:      producer,
	}
}

type NotificationSendEvent struct {
	Recipient string            `json:"recipient"`
	Template  string            `json:"template"`
	Data      map[string]string `json:"data"`
}

func (s *InvoiceService) GenerateAsync(ctx context.Context, order *model.Order) {
	// Execute inside goroutine, catch any panic/error to prevent server crash
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("panic recovered in GenerateAsync: %v\n", r)
		}
	}()

	// 1. Fetch order items
	items, err := s.orderItemRepo.GetByOrderID(ctx, order.ID.String())
	if err != nil {
		fmt.Printf("invoice: failed to fetch order items: %v\n", err)
		return
	}
	order.Items = items

	// 2. Format S3 invoice URL
	now := time.Now()
	year := now.Year()
	month := int(now.Month())
	invoiceURL := fmt.Sprintf("https://zippyra-invoices.s3.ap-south-1.amazonaws.com/invoices/%s/%d/%02d/%s.pdf",
		order.StoreID.String(), year, month, order.ID.String())

	// 3. Update orders table with invoice URL
	err = s.orderRepo.UpdateInvoiceURL(ctx, order.ID, invoiceURL)
	if err != nil {
		fmt.Printf("invoice: failed to update invoice URL in DB: %v\n", err)
		return
	}

	// 4. Publish notification.send event
	notifyEvent := NotificationSendEvent{
		Recipient: fmt.Sprintf("user_%s@zippyra.com", order.UserID.String()), // Dummy user contact email
		Template:  "invoice_template",
		Data: map[string]string{
			"order_number": order.OrderNumber,
			"invoice_url":  invoiceURL,
		},
	}

	err = s.producer.Publish(ctx, "notification.send", order.ID.String(), notifyEvent)
	if err != nil {
		fmt.Printf("invoice: failed to publish notification.send event: %v\n", err)
		return
	}
}
