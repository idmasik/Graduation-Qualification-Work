// File: analysis.go
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"
)

type outStruct struct {
	Zone           string `json:"zone"`
	Path           string `json:"path"`
	Md5            string `json:"md5"`
	DetectionName  string `json:"detection_name"`
	LastDetectDate string `json:"last_detect_date"`
	Error          string `json:"error,omitempty"`
}

// Client wraps an HTTP client and API key for OpenTIP.
type Client struct {
	httpClient *http.Client
	apiKey     string
}

// HashResponse represents the fields returned by OpenTIP for a hash lookup.
type HashResponse struct {
	Zone           string `json:"Zone"`
	DetectionName  string `json:"DetectionName"`
	LastDetectDate string `json:"LastDetectDate"`
}

// NewClient creates a new OpenTIP client, reading the API key from the environment.
func NewClient(apiKey string) (*Client, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("KASPERSKY_API_KEY not set")
	}
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		apiKey:     apiKey,
	}, nil
}

// LookupHash queries OpenTIP for the given MD5 hash and returns the parsed response.
func (c *Client) LookupHash(hash string) (*HashResponse, error) {
	url := fmt.Sprintf("https://opentip.kaspersky.com/api/v1/search/hash?request=%s", hash)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("X-API-KEY", c.apiKey)
	// if dump, derr := httputil.DumpRequestOut(req, false); derr == nil {
	// 	logger.Log(LevelDebug, fmt.Sprintf("HTTP Request:\n%s", dump))
	// }

	var resp *http.Response
	var attemptErr error
	for i := 1; i <= 3; i++ {
		resp, attemptErr = c.httpClient.Do(req)
		if attemptErr != nil {
			var netErr net.Error
			if errors.As(attemptErr, &netErr) && netErr.Timeout() {
				logger.Log(LevelWarning, fmt.Sprintf("Timeout on attempt %d for %s, retrying...", i, hash))
				time.Sleep(time.Duration(i) * time.Second)
				continue
			}
			return nil, attemptErr
		}
		break
	}
	if attemptErr != nil {
		return nil, fmt.Errorf("after retries: %w", attemptErr)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	logger.Log(LevelDebug, fmt.Sprintf("OpenTIP Response for %s:\n%s", hash, body))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %s", resp.Status)
	}

	var hr HashResponse
	if err := json.Unmarshal(body, &hr); err != nil {
		return nil, err
	}
	return &hr, nil
}

// AnalysisQueue manages a buffered queue of artifacts to analyze.
type AnalysisQueue struct {
	client      *Client
	queue       chan map[string]interface{}
	resultsFile *os.File
}

// NewQueue creates a new AnalysisQueue with the given buffer size, number of workers, and output file.
func NewQueue(client *Client, bufferSize, workers int, resultsPath string) (*AnalysisQueue, error) {
	f, err := os.OpenFile(resultsPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}
	q := &AnalysisQueue{
		client:      client,
		queue:       make(chan map[string]interface{}, bufferSize),
		resultsFile: f,
	}
	for i := 0; i < workers; i++ {
		go q.worker()
	}
	return q, nil
}

// Enqueue adds a new artifact info to the analysis queue without blocking.
func (q *AnalysisQueue) Enqueue(info map[string]interface{}) {
	select {
	case q.queue <- info:
	default:
		logger.Log(LevelWarning, fmt.Sprintf("Analysis queue full, dropping artifact with hash %v", info))
	}
}

// Close shuts down the queue and closes the results file.
func (q *AnalysisQueue) Close() error {
	close(q.queue)
	return q.resultsFile.Close()
}

// worker consumes items from the queue, looks them up in OpenTIP, and writes ordered JSON results.
func (q *AnalysisQueue) worker() {
	for info := range q.queue {
		rawFile, ok := info["file"]
		if !ok {
			continue
		}

		// Normalize to map[string]interface{}
		var fileMap map[string]interface{}
		switch f := rawFile.(type) {
		case map[string]interface{}:
			fileMap = f
		case map[string]string:
			fileMap = make(map[string]interface{}, len(f))
			for k, v := range f {
				fileMap[k] = v
			}
		default:
			continue
		}

		// Extract MD5
		var md5 string
		if h, ok := fileMap["hash"]; ok {
			switch hm := h.(type) {
			case map[string]interface{}:
				md5, _ = hm["md5"].(string)
			case map[string]string:
				md5 = hm["md5"]
			}
		}

		// Extract MIME type
		mime, ok := fileMap["mime_type"].(string)
		if !ok {
			continue
		}

		if md5 == "" || !(mime == "application/x-msdownload" || mime == "application/vnd.microsoft.portable-executable") {
			continue
		}

		hr, err := q.client.LookupHash(md5)
		// Prepare result structure, including error if occurred

		res := outStruct{
			Path: fmt.Sprintf("%v", fileMap["path"]),
			Md5:  md5,
		}
		if err != nil {
			logger.Log(LevelError, fmt.Sprintf("Lookup error for %s: %v", md5, err))
			res.Error = err.Error()
		} else {
			res.Zone = hr.Zone
			res.DetectionName = hr.DetectionName
			res.LastDetectDate = hr.LastDetectDate
		}

		b, merr := json.Marshal(res)
		if merr != nil {
			logger.Log(LevelError, fmt.Sprintf("Failed to marshal result: %v", merr))
			continue
		}
		if _, werr := q.resultsFile.Write(append(b, '\n')); werr != nil {
			logger.Log(LevelError, fmt.Sprintf("Failed to write result: %v", werr))
		}
	}
}
