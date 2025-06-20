package confluence

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Client struct {
	domain     string
	user       string
	apiToken   string
	useBearer  bool
	httpClient http.Client
}

func NewClient(domain, user, apiToken string) *Client {
	return &Client{
		domain:     domain,
		user:       user,
		apiToken:   apiToken,
		useBearer:  false,
		httpClient: http.Client{},
	}
}
func NewClientWithBearer(domain, token string) *Client {
	return &Client{
		domain:     domain,
		apiToken:   token,
		useBearer:  true,
		httpClient: http.Client{},
	}
}

func (c *Client) do(req *http.Request) (*http.Response, error) {
	req.Host = c.domain
	req.URL.Host = c.domain
	req.URL.Scheme = "https"
	if c.useBearer {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiToken))
	} else {

		req.SetBasicAuth(c.user, c.apiToken)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	return c.httpClient.Do(req)
}

func (c *Client) doWithResult(req *http.Request, result any) error {
	resp, err := c.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return responseError(resp)
	}

	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(result)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) Get(url string, result any) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	return c.doWithResult(req, result)
}

func (c *Client) Put(url string, body any, result any) error {
	jsonBody, err := jsonEncodedBody(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPut, url, jsonBody)
	if err != nil {
		return err
	}
	return c.doWithResult(req, result)
}

func (c *Client) delete(url string) error {
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	resp, err := c.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 {
		return responseError(resp)
	}
	return nil
}

func jsonEncodedBody(body any) (io.Reader, error) {
	bodyBuf := new(bytes.Buffer)
	encoder := json.NewEncoder(bodyBuf)
	err := encoder.Encode(body)
	return bodyBuf, err
}

func responseError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("HTTP Error: %s: %s", resp.Status, body)
}

func (c *Client) Domain() string {
	return c.domain
}
