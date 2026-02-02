// Package secrets provides AWS Secrets Manager integration for Chronos.
package secrets

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	secretsmanagerTypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
)

// AWSSecretsManagerConfig configures the AWS Secrets Manager provider.
type AWSSecretsManagerConfig struct {
	// Region is the AWS region.
	Region string `json:"region" yaml:"region"`

	// AccessKeyID is the AWS access key ID (optional if using IAM role).
	AccessKeyID string `json:"access_key_id,omitempty" yaml:"access_key_id,omitempty"`

	// SecretAccessKey is the AWS secret access key (optional if using IAM role).
	SecretAccessKey string `json:"secret_access_key,omitempty" yaml:"secret_access_key,omitempty"`

	// SessionToken is the AWS session token for temporary credentials.
	SessionToken string `json:"session_token,omitempty" yaml:"session_token,omitempty"`

	// RoleARN is the ARN of the IAM role to assume.
	RoleARN string `json:"role_arn,omitempty" yaml:"role_arn,omitempty"`

	// Endpoint is a custom endpoint URL (for testing with LocalStack).
	Endpoint string `json:"endpoint,omitempty" yaml:"endpoint,omitempty"`

	// CacheTTL is how long to cache secrets (0 = no caching).
	CacheTTL time.Duration `json:"cache_ttl" yaml:"cache_ttl"`

	// Prefix is a prefix to prepend to all secret names.
	Prefix string `json:"prefix,omitempty" yaml:"prefix,omitempty"`
}

// DefaultAWSSecretsManagerConfig returns default configuration.
func DefaultAWSSecretsManagerConfig() AWSSecretsManagerConfig {
	return AWSSecretsManagerConfig{
		Region:   "us-east-1",
		CacheTTL: 5 * time.Minute,
	}
}

// AWSSecretsManagerProvider provides secrets from AWS Secrets Manager.
type AWSSecretsManagerProvider struct {
	client  *secretsmanager.Client
	config  AWSSecretsManagerConfig
	cache   map[string]*cachedSecret
	cacheMu sync.RWMutex
}

// NewAWSSecretsManagerProvider creates a new AWS Secrets Manager provider.
func NewAWSSecretsManagerProvider(ctx context.Context, cfg AWSSecretsManagerConfig) (*AWSSecretsManagerProvider, error) {
	// Build AWS config options
	var opts []func(*config.LoadOptions) error

	if cfg.Region != "" {
		opts = append(opts, config.WithRegion(cfg.Region))
	}

	// Use explicit credentials if provided
	if cfg.AccessKeyID != "" && cfg.SecretAccessKey != "" {
		opts = append(opts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(
				cfg.AccessKeyID,
				cfg.SecretAccessKey,
				cfg.SessionToken,
			),
		))
	}

	awsCfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create client with custom endpoint if specified
	clientOpts := []func(*secretsmanager.Options){}
	if cfg.Endpoint != "" {
		clientOpts = append(clientOpts, func(o *secretsmanager.Options) {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		})
	}

	client := secretsmanager.NewFromConfig(awsCfg, clientOpts...)

	return &AWSSecretsManagerProvider{
		client: client,
		config: cfg,
		cache:  make(map[string]*cachedSecret),
	}, nil
}

// Name returns the provider name.
func (p *AWSSecretsManagerProvider) Name() string {
	return "aws"
}

// GetSecret retrieves a secret from AWS Secrets Manager.
func (p *AWSSecretsManagerProvider) GetSecret(ctx context.Context, key string) (string, error) {
	// Apply prefix
	secretName := key
	if p.config.Prefix != "" {
		secretName = p.config.Prefix + key
	}

	// Check cache first
	if p.config.CacheTTL > 0 {
		p.cacheMu.RLock()
		if cached, ok := p.cache[secretName]; ok && time.Now().Before(cached.expiresAt) {
			p.cacheMu.RUnlock()
			return cached.value, nil
		}
		p.cacheMu.RUnlock()
	}

	// Parse key format: secret-name or secret-name#field
	name, field := parseSecretKey(key)
	if p.config.Prefix != "" {
		name = p.config.Prefix + name
	}

	// Get secret from AWS
	input := &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(name),
	}

	result, err := p.client.GetSecretValue(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to get secret %s: %w", name, err)
	}

	var value string
	if result.SecretString != nil {
		secretString := *result.SecretString

		// If field is specified, parse as JSON and extract field
		if field != "" {
			var secretData map[string]interface{}
			if err := json.Unmarshal([]byte(secretString), &secretData); err != nil {
				return "", fmt.Errorf("failed to parse secret as JSON: %w", err)
			}
			fieldValue, ok := secretData[field]
			if !ok {
				return "", fmt.Errorf("field %s not found in secret %s", field, name)
			}
			value = fmt.Sprintf("%v", fieldValue)
		} else {
			value = secretString
		}
	} else if result.SecretBinary != nil {
		value = string(result.SecretBinary)
	}

	// Cache the value
	if p.config.CacheTTL > 0 {
		p.cacheMu.Lock()
		p.cache[secretName] = &cachedSecret{
			value:     value,
			expiresAt: time.Now().Add(p.config.CacheTTL),
		}
		p.cacheMu.Unlock()
	}

	return value, nil
}

// ListSecrets lists secrets with the given prefix.
func (p *AWSSecretsManagerProvider) ListSecrets(ctx context.Context, prefix string) ([]string, error) {
	fullPrefix := prefix
	if p.config.Prefix != "" {
		fullPrefix = p.config.Prefix + prefix
	}

	var secrets []string
	var nextToken *string

	for {
		input := &secretsmanager.ListSecretsInput{
			MaxResults: aws.Int32(100),
			NextToken:  nextToken,
		}

		// Add filter if prefix is specified
		if fullPrefix != "" {
			input.Filters = []secretsmanagerTypes.Filter{
				{
					Key:    secretsmanagerTypes.FilterNameStringTypeName,
					Values: []string{fullPrefix},
				},
			}
		}

		result, err := p.client.ListSecrets(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("failed to list secrets: %w", err)
		}

		for _, secret := range result.SecretList {
			name := *secret.Name
			// Remove provider prefix before returning
			if p.config.Prefix != "" {
				name = strings.TrimPrefix(name, p.config.Prefix)
			}
			secrets = append(secrets, name)
		}

		nextToken = result.NextToken
		if nextToken == nil {
			break
		}
	}

	return secrets, nil
}

// Close cleans up resources.
func (p *AWSSecretsManagerProvider) Close(ctx context.Context) error {
	p.cacheMu.Lock()
	p.cache = make(map[string]*cachedSecret)
	p.cacheMu.Unlock()
	return nil
}

// ClearCache clears the secret cache.
func (p *AWSSecretsManagerProvider) ClearCache() {
	p.cacheMu.Lock()
	p.cache = make(map[string]*cachedSecret)
	p.cacheMu.Unlock()
}

// RotateSecret forces a refresh of a cached secret.
func (p *AWSSecretsManagerProvider) RotateSecret(key string) {
	secretName := key
	if p.config.Prefix != "" {
		secretName = p.config.Prefix + key
	}

	p.cacheMu.Lock()
	delete(p.cache, secretName)
	p.cacheMu.Unlock()
}

// Ensure AWSSecretsManagerProvider implements Provider interface.
var _ Provider = (*AWSSecretsManagerProvider)(nil)
