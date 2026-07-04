package main

import (
"fmt"
"log"
"net/http"
)

func main() {
	fmt.Println("📋 Zippyra Order Service starting...")
	log.Fatal(http.ListenAndServe(":8006", nil))
}
