package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gpu-compute-orchestrator/pkg/compute"
	"gpu-compute-orchestrator/pkg/mqtt"
	"gpu-compute-orchestrator/pkg/nomad"
)

var version = "dev"

type Server struct {
	computeMgr *compute.Manager
	httpServer *http.Server
}

func NewServer(computeMgr *compute.Manager, port string) *Server {
	mux := http.NewServeMux()

	srv := &Server{
		computeMgr: computeMgr,
		httpServer: &http.Server{
			Addr:         ":" + port,
			Handler:      mux,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
		},
	}

	// Routes
	mux.HandleFunc("/health", srv.handleHealth)
	mux.HandleFunc("/compute", srv.handleCompute)
	mux.HandleFunc("/compute/sync", srv.handleComputeSync)
	mux.HandleFunc("/jobs", srv.handleListJobs)
	mux.HandleFunc("/jobs/", srv.handleJobDetails)
	mux.HandleFunc("/jobs/cancel", srv.handleCancelJob)

	// Serve static files (web UI)
	mux.Handle("/", http.FileServer(http.Dir("./static")))

	return srv
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ok",
		"version": version,
		"time":    time.Now().Format(time.RFC3339),
	})
}

func (s *Server) handleCompute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req compute.Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	job, err := s.computeMgr.Submit(r.Context(), &req)
	if err != nil {
		http.Error(w, fmt.Sprintf("Job submission failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(job)
}

func (s *Server) handleComputeSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req compute.Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	job, err := s.computeMgr.SubmitAndWait(r.Context(), &req)
	if err != nil {
		http.Error(w, fmt.Sprintf("Job failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if job.Status == "completed" {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusInternalServerError)
	}
	json.NewEncoder(w).Encode(job)
}

func (s *Server) handleListJobs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	jobs := s.computeMgr.ListJobs()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"jobs":  jobs,
		"count": len(jobs),
	})
}

func (s *Server) handleJobDetails(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract job ID from path
	jobID := r.URL.Path[len("/jobs/"):]
	if jobID == "" {
		http.Error(w, "Job ID required", http.StatusBadRequest)
		return
	}

	job, exists := s.computeMgr.GetJob(jobID)
	if !exists {
		http.Error(w, "Job not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}

func (s *Server) handleCancelJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	jobID := r.URL.Query().Get("job_id")
	if jobID == "" {
		http.Error(w, "job_id parameter required", http.StatusBadRequest)
		return
	}

	if err := s.computeMgr.CancelJob(jobID); err != nil {
		http.Error(w, fmt.Sprintf("Cancel failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "cancelled",
		"job_id": jobID,
	})
}

func (s *Server) Start() error {
	log.Printf("Starting HTTP server on %s (version: %s)", s.httpServer.Addr, version)
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	log.Println("Shutting down HTTP server...")
	return s.httpServer.Shutdown(ctx)
}

func main() {
	log.Printf("GPU Compute Orchestrator %s", version)

	// Configuration from environment
	mqttBroker := getEnv("MQTT_BROKER", "tcp://mqtt:1883")
	nomadAddr := getEnv("NOMAD_ADDR", "http://nomad:4646")
	nomadJobName := getEnv("NOMAD_JOB_NAME", "gpu-compute")
	port := getEnv("PORT", "8080")

	// Initialize MQTT client
	mqttClient, err := mqtt.NewClient(&mqtt.Config{
		Broker: mqttBroker,
		QoS:    1,
	})
	if err != nil {
		log.Fatalf("Failed to create MQTT client: %v", err)
	}
	defer mqttClient.Disconnect()

	// Initialize Nomad dispatcher
	nomadDispatcher, err := nomad.NewDispatcher(&nomad.Config{
		Address: nomadAddr,
		JobName: nomadJobName,
	})
	if err != nil {
		log.Fatalf("Failed to create Nomad dispatcher: %v", err)
	}

	// Initialize compute manager
	computeMgr := compute.NewManager(mqttClient, nomadDispatcher, &compute.Config{
		DefaultTimeout: 5 * time.Minute,
		MaxRetries:     3,
	})

	// Create and start server
	server := NewServer(computeMgr, port)

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start server in goroutine
	go func() {
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	log.Println("Server started successfully")

	// Wait for shutdown signal
	<-sigChan
	log.Println("Received shutdown signal")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	log.Println("Shutdown complete")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
