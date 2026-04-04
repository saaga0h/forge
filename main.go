package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gpu-compute-orchestrator/pkg/compute"
	"gpu-compute-orchestrator/pkg/mqtt"
	"gpu-compute-orchestrator/pkg/nomad"
)

var version = "dev"

func main() {
	log.Printf("GPU Compute Orchestrator %s", version)

	// Configuration from environment
	mqttBroker := getEnv("MQTT_BROKER", "tcp://mqtt:1883")
	mqttUser := getEnv("MQTT_USER", "")
	mqttPassword := getEnv("MQTT_PASSWORD", "")
	nomadAddr := getEnv("NOMAD_ADDR", "http://nomad:4646")

	// Initialize MQTT client
	mqttClient, err := mqtt.NewClient(&mqtt.Config{
		Broker:   mqttBroker,
		Username: mqttUser,
		Password: mqttPassword,
		QoS:      1,
	})
	if err != nil {
		log.Fatalf("Failed to create MQTT client: %v", err)
	}
	defer mqttClient.Disconnect()

	// Initialize Nomad dispatcher
	nomadDispatcher, err := nomad.NewDispatcher(&nomad.Config{
		Address: nomadAddr,
	})
	if err != nil {
		log.Fatalf("Failed to create Nomad dispatcher: %v", err)
	}

	// Initialize compute manager
	computeMgr := compute.NewManager(mqttClient, nomadDispatcher, &compute.Config{
		DefaultTimeout: 5 * time.Minute,
	})
	_ = computeMgr

	log.Println("Forge started successfully")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutdown complete")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
