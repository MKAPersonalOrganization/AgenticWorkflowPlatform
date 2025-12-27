package main

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"

	"school-with-ai/components"
	"school-with-ai/handlers"
	"school-with-ai/internal/session"
	openaiservice "school-with-ai/services/openai"
)

func main() {
	addr := os.Getenv("SERVER_ADDR")
	if addr == "" {
		panic("SERVER_ADDR environment variable is not set")
	}

	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		panic(fmt.Sprintf("invalid SERVER_ADDR: %v", err))
	}

	slog.Info("Starting server", "host", host, "port", port)

	// Initialize session store
	store := session.NewStore()

	// Initialize OpenAI client
	aiClient, err := openaiservice.NewClient()
	if err != nil {
		slog.Warn("Failed to initialize OpenAI client, will use fallback", "error", err)
		// Continue without OpenAI - the handler will need to handle nil client
	}

	mux := http.NewServeMux()

	// Serve static assets
	mux.Handle("/assets/css/", http.StripPrefix("/assets/css/", http.FileServer(http.Dir("./assets/css"))))
	mux.Handle("/assets/js/", http.StripPrefix("/assets/js/", http.FileServer(http.Dir("./assets/js"))))
	mux.Handle("/assets/images/", http.StripPrefix("/assets/images/", http.FileServer(http.Dir("./assets/images"))))

	// Chat routes
	mux.HandleFunc("/send", handlers.HandleSendMessage(store, aiClient))
	mux.HandleFunc("GET /api/threads", handlers.HandleGetThreads(store))
	mux.HandleFunc("GET /api/threads/{threadId}/messages", handlers.HandleGetMessages(store))
	mux.HandleFunc("DELETE /api/threads/{threadId}", handlers.HandleDeleteThread(store))
	mux.HandleFunc("/", HandleViewChatInputScreen)
	err = http.ListenAndServe(addr, mux)
	if err != nil {
		panic(err)
	}
}

func HandleViewChatInputScreen(w http.ResponseWriter, r *http.Request) {
	component := components.ChatInputScreen()
	err := component.Render(r.Context(), w)
	if err != nil {
		slog.Error("Failed to render chat screen", "error", err)
		// Don't call http.Error after Render has already written headers
		// Just log the error - the partial response will be sent
	}
}
