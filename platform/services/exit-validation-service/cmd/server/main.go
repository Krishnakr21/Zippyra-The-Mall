package main

import (
"fmt"
"log"
"net/http"
)

func main() {
	fmt.Println("🚪 Zippyra Exit Validation Service starting...")
	log.Fatal(http.ListenAndServe(":8009", nil))
}
