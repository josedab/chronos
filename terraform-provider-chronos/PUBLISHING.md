# Terraform Registry Publication Guide

## Prerequisites

1. Public GitHub repository
2. GPG key for provider signing
3. GitHub repository secrets configured

## Step 1: Generate GPG Key

```bash
gpg --full-generate-key
# Select RSA, 4096 bits, no expiration
# Use "Chronos Team <security@chronos.dev>"

# Export the private key
gpg --armor --export-secret-keys security@chronos.dev > gpg-private.key

# Get the fingerprint
gpg --list-secret-keys --keyid-format=long
```

## Step 2: Configure GitHub Secrets

Add to repository Settings → Secrets:
- `GPG_PRIVATE_KEY`: Contents of `gpg-private.key`
- `GPG_PASSPHRASE`: The passphrase used during key generation

## Step 3: Tag and Release

```bash
cd terraform-provider-chronos
git tag -a terraform-provider-v0.1.0 -m "Terraform Provider v0.1.0"
git push origin terraform-provider-v0.1.0
```

The GitHub Actions workflow (`.github/workflows/terraform-release.yml`) will automatically:
1. Build the provider for all platforms
2. Sign the release with GPG
3. Create a GitHub Release

## Step 4: Register on Terraform Registry

1. Go to https://registry.terraform.io/publish/provider
2. Select the `chronos/terraform-provider-chronos` repository
3. The registry will detect the manifest and signed releases
4. Provider will be available at `registry.terraform.io/chronos/chronos`

## Verification

```bash
terraform init
# Should download from registry.terraform.io/chronos/chronos
```

## Files Required (all present ✅)

- `terraform-registry-manifest.json` — Protocol version declaration
- `.goreleaser.yml` — Multi-platform build configuration
- `docs/resources/*.md` — Resource documentation (2 files)
- `docs/data-sources/*.md` — Data source documentation (1 file)
- `main.go` — Provider entry point
- `.github/workflows/terraform-release.yml` — CI/CD workflow
