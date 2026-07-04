package main

import (
"fmt"
"log"
"net/http"
)

func main() {
	fmt.Println("🎧 Zippyra Support Service starting...")
	log.Fatal(http.ListenAndServe(":8012", nil))
}
