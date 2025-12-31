package cli

import (
	"fmt"

	"github.com/pay-theory/lift/internal/liftconfig"
)

func resolveDeployContext(cfg *liftconfig.Config, partner, targetMode string) (resolvedPartner, resolvedTargetMode string, err error) {
	requiresPartner := cdkTemplatesUsePartner(cfg)
	requiresTargetMode := cdkTemplatesUseTargetMode(cfg)

	if requiresPartner && partner == "" {
		return "", "", fmt.Errorf("--partner is required for this project (cdk stack name_template references {{.Partner}})")
	}

	if targetMode == "" && (partner != "" || requiresTargetMode) {
		targetMode = defaultTargetMode
	}

	return partner, targetMode, nil
}
