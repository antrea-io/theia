package infra

import (
	"fmt"
)

func KmsSecretsProviderURL(keyID string, region string) string {
	return fmt.Sprintf("awskms://%s?region=%s", keyID, region)
}
