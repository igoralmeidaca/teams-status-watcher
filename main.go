package main

import (
	"bufio"
	"bytes"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

var (
	homeAssistantURL   = os.Getenv("HOME_ASSISTANT_URL")
	homeAssistantToken = os.Getenv("HOME_ASSISTANT_TOKEN")
	mqttBroker         = os.Getenv("MQTT_BROKER")
	mqttTopic          = os.Getenv("MQTT_TOPIC")
	mqttUsername       = os.Getenv("MQTT_USERNAME")
	mqttPassword       = os.Getenv("MQTT_PASSWORD")
	logsPath           = os.Getenv("LOGS_PATH")
)

func main() {
	var lastStatus string

	// Initialize MQTT client
	opts := mqtt.NewClientOptions().AddBroker(mqttBroker)
	opts.SetClientID("teams-status-watcher")
	opts.SetUsername(mqttUsername)
	opts.SetPassword(mqttPassword)

	mqttClient := mqtt.NewClient(opts)
	if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
		fmt.Println("Failed to connect to MQTT broker:", token.Error())
		return
	}
	defer mqttClient.Disconnect(250)

	for {
		latestLogFile, err := getLatestLogFile(logsPath)
		if err != nil {
			fmt.Println("Error finding log file:", err)
			time.Sleep(5 * time.Second)
			continue
		}

		lastAvailability, err := getLastAvailability(latestLogFile)
		if err != nil {
			fmt.Println("Error reading log file:", err)
			time.Sleep(5 * time.Second)
			continue
		}

		if lastAvailability != "" && lastAvailability != lastStatus {
			fmt.Println("New Teams status:", lastAvailability)
			updateHomeAssistant(lastAvailability)
			publishMQTT(mqttClient, lastAvailability)
			lastStatus = lastAvailability
		} else if lastStatus == "" {
			updateHomeAssistant("Error")
			publishMQTT(mqttClient, "Error")
			lastStatus = "Error"
		}

		time.Sleep(2 * time.Second)
	}
}

// Get the most recent log file based on filename timestamp
func getLatestLogFile(directory string) (string, error) {
	files, err := filepath.Glob(filepath.Join(directory, "MSTeams_*.log"))
	if err != nil || len(files) == 0 {
		return "", fmt.Errorf("no log files found")
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i] > files[j] // Sort descending to get the latest file first
	})

	return files[0], nil
}

// Get the last occurrence of "availability" in the log file
func getLastAvailability(filename string) (string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()

	var lastAvailability string
	re := regexp.MustCompile(`availability:\s+(\w+)`)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		match := re.FindStringSubmatch(line)
		if len(match) > 1 {
			lastAvailability = match[1] // Keep updating until the last occurrence
		}
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return lastAvailability, nil
}

// Send the availability status to Home Assistant
func updateHomeAssistant(status string) {
	jsonBody := fmt.Sprintf(`{"state": "%s", "attributes": {"friendly_name": "Teams Status"}}`, status)
	req, err := http.NewRequest("POST", homeAssistantURL, bytes.NewBuffer([]byte(jsonBody)))
	if err != nil {
		fmt.Println("Error creating request:", err)
		return
	}

	req.Header.Set("Authorization", "Bearer "+homeAssistantToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error sending request to Home Assistant:", err)
		return
	}
	defer resp.Body.Close()

	fmt.Println("Home Assistant updated with status:", status)
}

// Publish Teams status to MQTT topic
func publishMQTT(client mqtt.Client, status string) {
	token := client.Publish(mqttTopic, 0, false, status)
	token.Wait()
	fmt.Println("Published to MQTT:", mqttTopic, "->", status)
}
