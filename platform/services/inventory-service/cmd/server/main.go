package main

import (
"fmt"
"log"
"net/http"
)

func main() {
	fmt.Println("📊 Zippyra Inventory Service starting...")
	log.Fatal(http.ListenAndServe(":8005", nil))
}
