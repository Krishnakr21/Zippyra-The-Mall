package main

import (
"fmt"
"log"
"net/http"
)

func main() {
	fmt.Println("⭐ Zippyra Loyalty Service starting...")
	log.Fatal(http.ListenAndServe(":8008", nil))
}
