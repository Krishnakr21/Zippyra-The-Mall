package main

import (
"fmt"
"log"
"net/http"
)

func main() {
	fmt.Println("🔔 Zippyra Notification Service starting...")
	log.Fatal(http.ListenAndServe(":8011", nil))
}
