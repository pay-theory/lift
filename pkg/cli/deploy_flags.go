package cli

import "strings"

const (
	defaultStage      = "dev"
	defaultTargetMode = "standard"
)

func parseDeployFlags(args []string) (stage, partner, targetMode string) {
	stage = defaultStage

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--stage" && i+1 < len(args):
			i++
			stage = args[i]
		case strings.HasPrefix(arg, "--stage="):
			stage = strings.TrimPrefix(arg, "--stage=")
		case arg == "--partner" && i+1 < len(args):
			i++
			partner = args[i]
		case strings.HasPrefix(arg, "--partner="):
			partner = strings.TrimPrefix(arg, "--partner=")
		case arg == "--target-mode" && i+1 < len(args):
			i++
			targetMode = args[i]
		case strings.HasPrefix(arg, "--target-mode="):
			targetMode = strings.TrimPrefix(arg, "--target-mode=")
		}
	}

	return stage, partner, targetMode
}
