package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Submission struct {
	Name      string `json:"name"`
	Phone     string `json:"phone"`
	LinkedIn  string `json:"linkedin,omitempty"`
	Timestamp string `json:"timestamp"`
}

type githubContentResponse struct {
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
	SHA      string `json:"sha"`
}

type githubUpdateRequest struct {
	Message   string `json:"message"`
	Content   string `json:"content"`
	Branch    string `json:"branch"`
	SHA       string `json:"sha,omitempty"`
	Committer struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	} `json:"committer"`
}

var (
	githubToken    string
	githubRepo     string
	githubFilePath string
	githubBranch   string
	serverPort     string
	allowOrigin    string
)

func loadConfigFromEnv() {
	githubToken = strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
	githubRepo = strings.TrimSpace(os.Getenv("GITHUB_REPO"))
	githubFilePath = strings.TrimSpace(os.Getenv("GITHUB_FILE_PATH"))
	githubBranch = strings.TrimSpace(os.Getenv("GITHUB_BRANCH"))
	serverPort = strings.TrimSpace(os.Getenv("PORT"))
	allowOrigin = strings.TrimSpace(os.Getenv("CORS_ALLOW_ORIGIN"))
}

func main() {
	// Load .env if present (non-fatal if missing)
	_ = godotenv.Load()

	loadConfigFromEnv()

	if serverPort == "" {
		serverPort = "8080"
	}
	if allowOrigin == "" {
		allowOrigin = "*"
	}

	if err := validateServerConfig(); err != nil {
		log.Fatalf("configuration error: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", withCORS(healthHandler))
	mux.HandleFunc("/api/login", withCORS(loginHandler))

	srv := &http.Server{
		Addr:         ":" + serverPort,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 20 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("Go API listening on http://localhost:%s", serverPort)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server error: %v", err)
	}
}

func validateServerConfig() error {
	if githubToken == "" {
		return errors.New("GITHUB_TOKEN is required")
	}
	if githubRepo == "" {
		return errors.New("GITHUB_REPO is required (e.g. owner/repo)")
	}
	if githubFilePath == "" {
		return errors.New("GITHUB_FILE_PATH is required (e.g. data/submissions.json)")
	}
	if githubBranch == "" {
		githubBranch = "main"
	}
	return nil
}

func withCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "600")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	io.WriteString(w, "{\"status\":\"ok\"}")
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var in struct {
		Name     string `json:"name"`
		Phone    string `json:"phone"`
		LinkedIn string `json:"linkedin"`
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&in); err != nil {
		http.Error(w, fmt.Sprintf("invalid json: %v", err), http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.Phone) == "" {
		http.Error(w, "name and phone are required", http.StatusBadRequest)
		return
	}

	submission := Submission{
		Name:      strings.TrimSpace(in.Name),
		Phone:     strings.TrimSpace(in.Phone),
		LinkedIn:  strings.TrimSpace(in.LinkedIn),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	if err := appendSubmissionToGitHub(r.Context(), submission); err != nil {
		log.Printf("append to GitHub failed: %v", err)
		http.Error(w, "failed to update GitHub file", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{
		"ok":      true,
		"message": "Submission saved",
	})
}

func appendSubmissionToGitHub(ctx context.Context, item Submission) error {
	// Fetch current file (if exists)
	content, sha, err := getGitHubFile(ctx)
	if err != nil {
		// If 404, start fresh array
		if errors.Is(err, errNotFound) {
			content = []byte("[]")
		} else {
			return err
		}
	}

	var arr []Submission
	if err := json.Unmarshal(content, &arr); err != nil {
		// If file isn't an array, wrap it
		arr = []Submission{}
	}
	arr = append(arr, item)

	updated, err := json.MarshalIndent(arr, "", "  ")
	if err != nil {
		return err
	}

	// PUT updated content
	return putGitHubFile(ctx, updated, sha)
}

var errNotFound = errors.New("github file not found")

func getGitHubFile(ctx context.Context) ([]byte, string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/contents/%s?ref=%s", githubRepo, githubFilePath, githubBranch)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Authorization", "Bearer "+githubToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "react-quiz-server-go")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, "", errNotFound
	}
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("github get failed: %s: %s", resp.Status, string(body))
	}

	var out githubContentResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, "", err
	}

	if out.Encoding != "base64" {
		return nil, "", fmt.Errorf("unexpected encoding: %s", out.Encoding)
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(out.Content, "\n", ""))
	if err != nil {
		return nil, "", err
	}
	return decoded, out.SHA, nil
}

func putGitHubFile(ctx context.Context, content []byte, sha string) error {
	url := fmt.Sprintf("https://api.github.com/repos/%s/contents/%s", githubRepo, githubFilePath)

	var body githubUpdateRequest
	body.Message = fmt.Sprintf("chore: append quiz login submission (%s)", time.Now().UTC().Format(time.RFC3339))
	body.Content = base64.StdEncoding.EncodeToString(content)
	body.Branch = githubBranch
	if sha != "" {
		body.SHA = sha
	}
	body.Committer.Name = "React Quiz Bot"
	body.Committer.Email = "noreply@example.com"

	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, strings.NewReader(string(payload)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+githubToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "react-quiz-server-go")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("github put failed: %s: %s", resp.Status, string(b))
	}
	return nil
}
