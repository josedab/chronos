// Package cloud provides cloud SDK integration for Chronos executors.
package cloud

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// AWSConfig contains AWS configuration.
type AWSConfig struct {
	Region          string `json:"region" yaml:"region"`
	AccessKeyID     string `json:"access_key_id" yaml:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key" yaml:"secret_access_key"`
	SessionToken    string `json:"session_token,omitempty" yaml:"session_token,omitempty"`
	RoleARN         string `json:"role_arn,omitempty" yaml:"role_arn,omitempty"`
	ExternalID      string `json:"external_id,omitempty" yaml:"external_id,omitempty"`
}

// AWSClient provides AWS API access with SigV4 signing.
type AWSClient struct {
	config     AWSConfig
	httpClient *http.Client
}

// NewAWSClient creates a new AWS client.
func NewAWSClient(cfg AWSConfig) *AWSClient {
	return &AWSClient{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// InvokeLambda invokes an AWS Lambda function.
func (c *AWSClient) InvokeLambda(ctx context.Context, functionName string, payload []byte, invocationType string) (*LambdaInvokeResult, error) {
	if invocationType == "" {
		invocationType = "RequestResponse"
	}

	endpoint := fmt.Sprintf("https://lambda.%s.amazonaws.com/2015-03-31/functions/%s/invocations",
		c.config.Region, url.PathEscape(functionName))

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Amz-Invocation-Type", invocationType)

	if err := c.signRequest(req, "lambda", payload); err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	result := &LambdaInvokeResult{
		StatusCode:      resp.StatusCode,
		Payload:         body,
		FunctionError:   resp.Header.Get("X-Amz-Function-Error"),
		LogResult:       resp.Header.Get("X-Amz-Log-Result"),
		ExecutedVersion: resp.Header.Get("X-Amz-Executed-Version"),
	}

	return result, nil
}

// LambdaInvokeResult contains Lambda invocation results.
type LambdaInvokeResult struct {
	StatusCode      int    `json:"status_code"`
	Payload         []byte `json:"payload"`
	FunctionError   string `json:"function_error,omitempty"`
	LogResult       string `json:"log_result,omitempty"`
	ExecutedVersion string `json:"executed_version,omitempty"`
}

// SendSQSMessage sends a message to an SQS queue.
func (c *AWSClient) SendSQSMessage(ctx context.Context, queueURL string, messageBody string, attributes map[string]string) (*SQSSendResult, error) {
	params := url.Values{}
	params.Set("Action", "SendMessage")
	params.Set("Version", "2012-11-05")
	params.Set("MessageBody", messageBody)

	i := 1
	for k, v := range attributes {
		params.Set(fmt.Sprintf("MessageAttribute.%d.Name", i), k)
		params.Set(fmt.Sprintf("MessageAttribute.%d.Value.DataType", i), "String")
		params.Set(fmt.Sprintf("MessageAttribute.%d.Value.StringValue", i), v)
		i++
	}

	body := []byte(params.Encode())
	req, err := http.NewRequestWithContext(ctx, "POST", queueURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	if err := c.signRequest(req, "sqs", body); err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("SQS error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return &SQSSendResult{
		MessageID: extractXMLValue(string(respBody), "MessageId"),
		MD5:       extractXMLValue(string(respBody), "MD5OfMessageBody"),
	}, nil
}

// SQSSendResult contains SQS send message results.
type SQSSendResult struct {
	MessageID string `json:"message_id"`
	MD5       string `json:"md5"`
}

// PublishSNS publishes a message to an SNS topic.
func (c *AWSClient) PublishSNS(ctx context.Context, topicARN string, message string, subject string) (*SNSPublishResult, error) {
	endpoint := fmt.Sprintf("https://sns.%s.amazonaws.com/", c.config.Region)

	params := url.Values{}
	params.Set("Action", "Publish")
	params.Set("Version", "2010-03-31")
	params.Set("TopicArn", topicARN)
	params.Set("Message", message)
	if subject != "" {
		params.Set("Subject", subject)
	}

	body := []byte(params.Encode())
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	if err := c.signRequest(req, "sns", body); err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("SNS error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return &SNSPublishResult{
		MessageID: extractXMLValue(string(respBody), "MessageId"),
	}, nil
}

// SNSPublishResult contains SNS publish results.
type SNSPublishResult struct {
	MessageID string `json:"message_id"`
}

// signRequest signs an AWS request using Signature Version 4.
func (c *AWSClient) signRequest(req *http.Request, service string, payload []byte) error {
	now := time.Now().UTC()
	dateStamp := now.Format("20060102")
	amzDate := now.Format("20060102T150405Z")

	req.Header.Set("X-Amz-Date", amzDate)
	if c.config.SessionToken != "" {
		req.Header.Set("X-Amz-Security-Token", c.config.SessionToken)
	}
	req.Header.Set("Host", req.URL.Host)

	canonicalURI := req.URL.Path
	if canonicalURI == "" {
		canonicalURI = "/"
	}

	canonicalQueryString := req.URL.RawQuery

	signedHeaders := "host;x-amz-date"
	canonicalHeaders := fmt.Sprintf("host:%s\nx-amz-date:%s\n", req.URL.Host, amzDate)
	if c.config.SessionToken != "" {
		signedHeaders = "host;x-amz-date;x-amz-security-token"
		canonicalHeaders = fmt.Sprintf("host:%s\nx-amz-date:%s\nx-amz-security-token:%s\n",
			req.URL.Host, amzDate, c.config.SessionToken)
	}

	payloadHash := sha256Hash(payload)

	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		req.Method,
		canonicalURI,
		canonicalQueryString,
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	)

	credentialScope := fmt.Sprintf("%s/%s/%s/aws4_request", dateStamp, c.config.Region, service)
	stringToSign := fmt.Sprintf("AWS4-HMAC-SHA256\n%s\n%s\n%s",
		amzDate,
		credentialScope,
		sha256Hash([]byte(canonicalRequest)),
	)

	signingKey := getSignatureKey(c.config.SecretAccessKey, dateStamp, c.config.Region, service)
	signature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))

	authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		c.config.AccessKeyID,
		credentialScope,
		signedHeaders,
		signature,
	)

	req.Header.Set("Authorization", authHeader)

	return nil
}

func sha256Hash(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func getSignatureKey(secret, dateStamp, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secret), []byte(dateStamp))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(service))
	kSigning := hmacSHA256(kService, []byte("aws4_request"))
	return kSigning
}

func extractXMLValue(xml, tag string) string {
	start := strings.Index(xml, "<"+tag+">")
	end := strings.Index(xml, "</"+tag+">")
	if start == -1 || end == -1 {
		return ""
	}
	return xml[start+len(tag)+2 : end]
}

// GCPConfig contains GCP configuration.
type GCPConfig struct {
	ProjectID       string `json:"project_id" yaml:"project_id"`
	Region          string `json:"region" yaml:"region"`
	CredentialsJSON string `json:"credentials_json,omitempty" yaml:"credentials_json,omitempty"`
	CredentialsFile string `json:"credentials_file,omitempty" yaml:"credentials_file,omitempty"`
}

// GCPClient provides GCP API access.
type GCPClient struct {
	config      GCPConfig
	httpClient  *http.Client
	accessToken string
	tokenExpiry time.Time
}

// NewGCPClient creates a new GCP client.
func NewGCPClient(cfg GCPConfig) *GCPClient {
	return &GCPClient{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// InvokeCloudRun invokes a Cloud Run service.
func (c *GCPClient) InvokeCloudRun(ctx context.Context, serviceURL string, payload []byte, headers map[string]string) (*CloudRunResult, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", serviceURL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	if c.accessToken != "" && time.Now().Before(c.tokenExpiry) {
		req.Header.Set("Authorization", "Bearer "+c.accessToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	return &CloudRunResult{
		StatusCode: resp.StatusCode,
		Body:       body,
		Headers:    flattenHeaders(resp.Header),
	}, nil
}

// CloudRunResult contains Cloud Run invocation results.
type CloudRunResult struct {
	StatusCode int               `json:"status_code"`
	Body       []byte            `json:"body"`
	Headers    map[string]string `json:"headers"`
}

// InvokeCloudFunction invokes a Cloud Function.
func (c *GCPClient) InvokeCloudFunction(ctx context.Context, functionName string, payload []byte) (*CloudFunctionResult, error) {
	endpoint := fmt.Sprintf("https://%s-%s.cloudfunctions.net/%s",
		c.config.Region, c.config.ProjectID, functionName)

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	if c.accessToken != "" && time.Now().Before(c.tokenExpiry) {
		req.Header.Set("Authorization", "Bearer "+c.accessToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	return &CloudFunctionResult{
		StatusCode: resp.StatusCode,
		Body:       body,
	}, nil
}

// CloudFunctionResult contains Cloud Function invocation results.
type CloudFunctionResult struct {
	StatusCode int    `json:"status_code"`
	Body       []byte `json:"body"`
}

// PublishPubSub publishes a message to Pub/Sub.
func (c *GCPClient) PublishPubSub(ctx context.Context, topic string, data []byte, attributes map[string]string) (*PubSubResult, error) {
	endpoint := fmt.Sprintf("https://pubsub.googleapis.com/v1/projects/%s/topics/%s:publish",
		c.config.ProjectID, topic)

	message := map[string]interface{}{
		"data": data,
	}
	if len(attributes) > 0 {
		message["attributes"] = attributes
	}

	payload := map[string]interface{}{
		"messages": []interface{}{message},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	if c.accessToken != "" && time.Now().Before(c.tokenExpiry) {
		req.Header.Set("Authorization", "Bearer "+c.accessToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Pub/Sub error (status %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		MessageIDs []string `json:"messageIds"`
	}
	json.Unmarshal(body, &result)

	messageID := ""
	if len(result.MessageIDs) > 0 {
		messageID = result.MessageIDs[0]
	}

	return &PubSubResult{MessageID: messageID}, nil
}

// PubSubResult contains Pub/Sub publish results.
type PubSubResult struct {
	MessageID string `json:"message_id"`
}

// AzureConfig contains Azure configuration.
type AzureConfig struct {
	SubscriptionID string `json:"subscription_id" yaml:"subscription_id"`
	ResourceGroup  string `json:"resource_group" yaml:"resource_group"`
	TenantID       string `json:"tenant_id" yaml:"tenant_id"`
	ClientID       string `json:"client_id" yaml:"client_id"`
	ClientSecret   string `json:"client_secret" yaml:"client_secret"`
}

// AzureClient provides Azure API access.
type AzureClient struct {
	config      AzureConfig
	httpClient  *http.Client
	accessToken string
	tokenExpiry time.Time
}

// NewAzureClient creates a new Azure client.
func NewAzureClient(cfg AzureConfig) *AzureClient {
	return &AzureClient{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Authenticate obtains an Azure access token.
func (c *AzureClient) Authenticate(ctx context.Context) error {
	endpoint := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", c.config.TenantID)

	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("client_id", c.config.ClientID)
	data.Set("client_secret", c.config.ClientSecret)
	data.Set("scope", "https://management.azure.com/.default")

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return err
	}

	c.accessToken = tokenResp.AccessToken
	c.tokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn-60) * time.Second)

	return nil
}

// InvokeFunction invokes an Azure Function.
func (c *AzureClient) InvokeFunction(ctx context.Context, functionAppURL string, functionName string, payload []byte) (*AzureFunctionResult, error) {
	endpoint := fmt.Sprintf("%s/api/%s", functionAppURL, functionName)

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	return &AzureFunctionResult{
		StatusCode: resp.StatusCode,
		Body:       body,
	}, nil
}

// AzureFunctionResult contains Azure Function invocation results.
type AzureFunctionResult struct {
	StatusCode int    `json:"status_code"`
	Body       []byte `json:"body"`
}

// SendEventGridEvent sends an event to Azure Event Grid.
func (c *AzureClient) SendEventGridEvent(ctx context.Context, topicEndpoint string, topicKey string, events []EventGridEvent) error {
	payload, err := json.Marshal(events)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", topicEndpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("aeg-sas-key", topicKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 202 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Event Grid error (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// EventGridEvent represents an Azure Event Grid event.
type EventGridEvent struct {
	ID          string                 `json:"id"`
	EventType   string                 `json:"eventType"`
	Subject     string                 `json:"subject"`
	EventTime   time.Time              `json:"eventTime"`
	Data        map[string]interface{} `json:"data"`
	DataVersion string                 `json:"dataVersion"`
}

// SendServiceBusMessage sends a message to Azure Service Bus.
func (c *AzureClient) SendServiceBusMessage(ctx context.Context, namespace, queue string, message []byte, properties map[string]string) error {
	endpoint := fmt.Sprintf("https://%s.servicebus.windows.net/%s/messages", namespace, queue)

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(message))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	if c.accessToken != "" && time.Now().Before(c.tokenExpiry) {
		req.Header.Set("Authorization", "Bearer "+c.accessToken)
	}

	for k, v := range properties {
		req.Header.Set("BrokerProperties", fmt.Sprintf(`{"%s":"%s"}`, k, v))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Service Bus error (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

func flattenHeaders(h http.Header) map[string]string {
	result := make(map[string]string)
	for k, v := range h {
		if len(v) > 0 {
			result[k] = v[0]
		}
	}
	return result
}
