package lift

import (
	"fmt"

	"github.com/pay-theory/dynamorm"
	"github.com/pay-theory/dynamorm/pkg/core"
	"github.com/pay-theory/dynamorm/pkg/session"
)

func initializeDynamORM(region, endpoint string, maxRetries int) (core.ExtendedDB, error) {
	sessionConfig := session.Config{
		Region: region,
	}
	if endpoint != "" {
		sessionConfig.Endpoint = endpoint
	}
	if maxRetries > 0 {
		sessionConfig.MaxRetries = maxRetries
	}

	db, err := dynamorm.New(sessionConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize DynamORM: %w", err)
	}
	return db, nil
}
