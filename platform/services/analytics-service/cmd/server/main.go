package main

import (
"fmt"
"log"
"net/http"
)

func main() {
	fmt.Println("📈 Zippyra Analytics Service starting...")
	log.Fatal(http.ListenAndServe(":8013", nil))
}
