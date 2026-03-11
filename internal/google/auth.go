package googleclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/youtube/v3"
)

type Authenticator struct {
	CredentialsFile string
	TokenFile       string
}

func NewAuthenticator(credentialsFile, tokenFile string) *Authenticator {
	return &Authenticator{
		CredentialsFile: credentialsFile,
		TokenFile:       tokenFile,
	}
}

func (a *Authenticator) Login(ctx context.Context) error {
	config, err := a.oauthConfig()
	if err != nil {
		return err
	}

	token, err := getTokenFromWeb(ctx, config)
	if err != nil {
		return err
	}

	if err := saveToken(a.TokenFile, token); err != nil {
		return err
	}

	fmt.Printf("OAuth login successful. Token saved to %s\n", a.TokenFile)
	return nil
}

func (a *Authenticator) NewHTTPClient(ctx context.Context) (*http.Client, error) {
	config, err := a.oauthConfig()
	if err != nil {
		return nil, err
	}

	token, err := tokenFromFile(a.TokenFile)
	if err != nil {
		return nil, fmt.Errorf("load OAuth token: %w; run 'eva-cli login' first", err)
	}

	return config.Client(ctx, token), nil
}

func (a *Authenticator) oauthConfig() (*oauth2.Config, error) {
	content, err := os.ReadFile(a.CredentialsFile)
	if err != nil {
		return nil, fmt.Errorf("read credentials file %q: %w", a.CredentialsFile, err)
	}

	config, err := google.ConfigFromJSON(content,
		drive.DriveReadonlyScope,
		youtube.YoutubeUploadScope,
	)
	if err != nil {
		return nil, fmt.Errorf("parse credentials file %q: %w", a.CredentialsFile, err)
	}

	return config, nil
}

func getTokenFromWeb(ctx context.Context, config *oauth2.Config) (*oauth2.Token, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("start OAuth callback listener: %w", err)
	}
	defer listener.Close()

	redirectURL := fmt.Sprintf("http://%s/callback", listener.Addr().String())
	config.RedirectURL = redirectURL

	state := fmt.Sprintf("eva-cli-%d", time.Now().UnixNano())
	authURL := config.AuthCodeURL(state, oauth2.AccessTypeOffline)
	fmt.Println("Open the following URL in your browser and authorize the application:")
	fmt.Println(authURL)
	fmt.Println()
	if err := openBrowser(authURL); err != nil {
		fmt.Printf("Unable to open the browser automatically: %v\n", err)
	}

	token, err := waitForOAuthCallback(ctx, listener, config, state)
	if err != nil {
		return nil, err
	}

	return token, nil
}

func waitForOAuthCallback(ctx context.Context, listener net.Listener, config *oauth2.Config, expectedState string) (*oauth2.Token, error) {
	resultCh := make(chan struct {
		token *oauth2.Token
		err   error
	}, 1)

	mux := http.NewServeMux()
	server := &http.Server{Handler: mux}
	mux.HandleFunc("/callback", func(writer http.ResponseWriter, request *http.Request) {
		query := request.URL.Query()
		if errValue := query.Get("error"); errValue != "" {
			http.Error(writer, "Google authorization failed. You can close this window.", http.StatusBadRequest)
			resultCh <- struct {
				token *oauth2.Token
				err   error
			}{err: fmt.Errorf("authorization failed: %s", errValue)}
			return
		}

		if query.Get("state") != expectedState {
			http.Error(writer, "Invalid OAuth state. You can close this window.", http.StatusBadRequest)
			resultCh <- struct {
				token *oauth2.Token
				err   error
			}{err: fmt.Errorf("invalid OAuth state returned by Google")}
			return
		}

		code := strings.TrimSpace(query.Get("code"))
		if code == "" {
			http.Error(writer, "Authorization code missing. You can close this window.", http.StatusBadRequest)
			resultCh <- struct {
				token *oauth2.Token
				err   error
			}{err: fmt.Errorf("authorization code missing from callback")}
			return
		}

		token, err := config.Exchange(request.Context(), code)
		if err != nil {
			http.Error(writer, "Token exchange failed. You can close this window.", http.StatusBadGateway)
			resultCh <- struct {
				token *oauth2.Token
				err   error
			}{err: fmt.Errorf("exchange authorization code: %w", err)}
			return
		}

		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = writer.Write([]byte("<html><body><h1>Login completed</h1><p>You can close this window and return to eva-cli.</p></body></html>"))
		resultCh <- struct {
			token *oauth2.Token
			err   error
		}{token: token}
	})

	go func() {
		_ = server.Serve(listener)
	}()

	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("OAuth login canceled: %w", ctx.Err())
	case result := <-resultCh:
		return result.token, result.err
	}
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

func tokenFromFile(path string) (*oauth2.Token, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var token oauth2.Token
	if err := json.Unmarshal(content, &token); err != nil {
		return nil, fmt.Errorf("parse token file %q: %w", path, err)
	}

	return &token, nil
}

func saveToken(path string, token *oauth2.Token) error {
	content, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal OAuth token: %w", err)
	}

	if err := os.WriteFile(path, content, 0o600); err != nil {
		return fmt.Errorf("write token file %q: %w", path, err)
	}

	return nil
}
