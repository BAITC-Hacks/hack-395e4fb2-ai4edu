package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"hack-395e4fb2-ai4edu/internal/advisor"
	"hack-395e4fb2-ai4edu/internal/explanation"
	"hack-395e4fb2-ai4edu/internal/httpapi"
	"hack-395e4fb2-ai4edu/internal/optimizer"
)

func main() {
	provider, reviewer, err := explanation.NewProvidersFromEnv()
	if err != nil {
		log.Fatal(err)
	}
	go func() {
		started := time.Now()
		log.Print("Computing optimal scenario")
		best := optimizer.Best()
		log.Printf("Optimal scenario ready in %s: score=%v cost=%d", time.Since(started), best.FinalScore, best.TotalCost)
	}()
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}
	server := &http.Server{
		Addr: addr, Handler: httpapi.NewHandlerWithOptions(httpapi.Options{
			Explainer:  explanation.New(provider),
			Advisor:    advisor.New(advisor.NewPlannerFromEnv(), reviewer),
			CORSOrigin: os.Getenv("CORS_ORIGIN"),
		}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		// Covers request read (10s), advisor deadline (45s), and response write.
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	log.Printf("Akim API listening on %s", addr)
	log.Fatal(server.ListenAndServe())
}
