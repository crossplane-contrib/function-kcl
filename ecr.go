package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
)

// getECRCredential gets a short-lived basic-auth credential for AWS ECR from the
// pod's own AWS identity (IRSA / EKS Pod Identity / the standard credential chain),
// so no static credential has to be stored. It returns the username ("AWS") and the
// password (the decoded ECR authorization token) that ORAS/kpm use to pull the OCI
// source. registry is the credentials `url` — an ECR registry host, optionally with a
// scheme or a path (e.g. "https://<account>.dkr.ecr.<region>.amazonaws.com").
func getECRCredential(ctx context.Context, registry string) (username, password string, err error) {
	region, err := regionFromECRRegistry(registry)
	if err != nil {
		return "", "", err
	}
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return "", "", fmt.Errorf("load AWS config: %w", err)
	}
	out, err := ecr.NewFromConfig(cfg).GetAuthorizationToken(ctx, &ecr.GetAuthorizationTokenInput{})
	if err != nil {
		return "", "", fmt.Errorf("ECR GetAuthorizationToken: %w", err)
	}
	if len(out.AuthorizationData) == 0 || out.AuthorizationData[0].AuthorizationToken == nil {
		return "", "", fmt.Errorf("ECR returned no authorization data")
	}
	decoded, err := base64.StdEncoding.DecodeString(*out.AuthorizationData[0].AuthorizationToken)
	if err != nil {
		return "", "", fmt.Errorf("decode ECR authorization token: %w", err)
	}
	// The token decodes to "AWS:<password>".
	user, pass, ok := strings.Cut(string(decoded), ":")
	if !ok {
		return "", "", fmt.Errorf("unexpected ECR authorization token format")
	}
	return user, pass, nil
}

// writeECRDockerConfig writes the fetched credential into a docker config.json and points DOCKER_CONFIG
// at it, so kpm's PULL reads the auth from the config store instead of us calling kpm login. kpm's
// login refuses to STORE plaintext credentials for an HTTPS registry ("putting plaintext credentials is
// disabled"); reading a plaintext `auths` entry for the pull is fine, so this sidesteps the login.
func writeECRDockerConfig(registry, username, password string) error {
	host, err := ecrHost(registry)
	if err != nil {
		return err
	}
	auth := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	cfg := map[string]any{"auths": map[string]any{host: map[string]string{"auth": auth}}}
	body, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	dir := filepath.Join(os.TempDir(), "kcl-ecr-docker")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), body, 0o600); err != nil {
		return err
	}
	return os.Setenv("DOCKER_CONFIG", dir)
}

// ecrHost strips any scheme and path from an ECR registry reference, leaving the bare host that keys
// the docker config `auths` map (e.g. "<account>.dkr.ecr.<region>.amazonaws.com").
func ecrHost(registry string) (string, error) {
	host := registry
	for _, scheme := range []string{"https://", "http://", "oci://"} {
		host = strings.TrimPrefix(host, scheme)
	}
	if i := strings.IndexByte(host, '/'); i >= 0 {
		host = host[:i]
	}
	if host == "" {
		return "", fmt.Errorf("empty registry host: %q", registry)
	}
	return host, nil
}

// regionFromECRRegistry pulls the region out of an ECR registry host of the form
// "<account>.dkr.ecr.<region>.amazonaws.com". GetAuthorizationToken has to be called against the
// registry's own region.
func regionFromECRRegistry(registry string) (string, error) {
	host, err := ecrHost(registry)
	if err != nil {
		return "", err
	}
	// <account> . dkr . ecr . <region> . amazonaws . com
	parts := strings.Split(host, ".")
	if len(parts) < 6 || parts[1] != "dkr" || parts[2] != "ecr" {
		return "", fmt.Errorf("not an ECR registry host: %q", registry)
	}
	return parts[3], nil
}
