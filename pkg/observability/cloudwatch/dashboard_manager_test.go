package cloudwatch

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/stretchr/testify/require"
)

type fakeCloudWatchDashboardClient struct {
	mu sync.Mutex

	putErr    error
	getErr    error
	listErr   error
	deleteErr error

	getOutput *cloudwatch.GetDashboardOutput
	listPages map[string]*cloudwatch.ListDashboardsOutput

	putInputs    []*cloudwatch.PutDashboardInput
	getInputs    []*cloudwatch.GetDashboardInput
	listTokens   []string
	deleteInputs []*cloudwatch.DeleteDashboardsInput
}

func (f *fakeCloudWatchDashboardClient) PutDashboard(_ context.Context, params *cloudwatch.PutDashboardInput, _ ...func(*cloudwatch.Options)) (*cloudwatch.PutDashboardOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	clone := *params
	f.putInputs = append(f.putInputs, &clone)
	if f.putErr != nil {
		return nil, f.putErr
	}
	return &cloudwatch.PutDashboardOutput{}, nil
}

func (f *fakeCloudWatchDashboardClient) GetDashboard(_ context.Context, params *cloudwatch.GetDashboardInput, _ ...func(*cloudwatch.Options)) (*cloudwatch.GetDashboardOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	clone := *params
	f.getInputs = append(f.getInputs, &clone)
	if f.getErr != nil {
		return nil, f.getErr
	}
	if f.getOutput != nil {
		return f.getOutput, nil
	}
	return &cloudwatch.GetDashboardOutput{}, nil
}

func (f *fakeCloudWatchDashboardClient) ListDashboards(_ context.Context, params *cloudwatch.ListDashboardsInput, _ ...func(*cloudwatch.Options)) (*cloudwatch.ListDashboardsOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.listErr != nil {
		return nil, f.listErr
	}
	token := aws.ToString(params.NextToken)
	f.listTokens = append(f.listTokens, token)
	if f.listPages == nil {
		return &cloudwatch.ListDashboardsOutput{}, nil
	}
	if page, ok := f.listPages[token]; ok {
		return page, nil
	}
	return &cloudwatch.ListDashboardsOutput{}, nil
}

func (f *fakeCloudWatchDashboardClient) DeleteDashboards(_ context.Context, params *cloudwatch.DeleteDashboardsInput, _ ...func(*cloudwatch.Options)) (*cloudwatch.DeleteDashboardsOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	clone := *params
	f.deleteInputs = append(f.deleteInputs, &clone)
	if f.deleteErr != nil {
		return nil, f.deleteErr
	}
	return &cloudwatch.DeleteDashboardsOutput{}, nil
}

func TestDashboardManager_RegisterTemplate(t *testing.T) {
	client := &fakeCloudWatchDashboardClient{}
	dm := NewDashboardManager(client, DashboardManagerConfig{
		Namespace:   "ns",
		Environment: "prod",
		Region:      "us-east-1",
	})

	require.Error(t, dm.RegisterTemplate(&DashboardTemplate{}))

	template := &DashboardTemplate{
		ID:        "t1",
		Name:      "template",
		Version:   "1",
		Variables: map[string]any{"service": "svc"},
	}
	require.True(t, template.CreatedAt.IsZero())
	require.NoError(t, dm.RegisterTemplate(template))
	require.False(t, template.CreatedAt.IsZero())
	require.False(t, template.UpdatedAt.IsZero())

	got, ok := dm.GetTemplate("t1")
	require.True(t, ok)
	require.Equal(t, "template", got.Name)
}

func TestDashboardManager_CreateUpdateDeleteAndGet(t *testing.T) {
	client := &fakeCloudWatchDashboardClient{}
	dm := NewDashboardManager(client, DashboardManagerConfig{
		Namespace:      "ns",
		Environment:    "prod",
		Region:         "us-east-1",
		UpdateInterval: 10 * time.Millisecond,
	})

	template := &DashboardTemplate{
		ID:      "t1",
		Version: "1.0.0",
		Variables: map[string]any{
			"service": "svc-template",
		},
		Widgets: []WidgetTemplate{{
			Type:  "metric",
			Title: "${service}-${environment}-${count}",
			Properties: map[string]any{
				"view": "timeSeries",
			},
			Metrics: []MetricDefinition{{
				Namespace:  "${namespace}",
				MetricName: "Requests",
				Dimensions: map[string]string{
					"Service": "${service}",
					"Env":     "${environment}",
				},
				Statistic: "Sum",
				Period:    60,
				Label:     "${service}-${environment}",
			}},
			Position: WidgetPosition{X: 1, Y: 2},
			Size:     WidgetSize{Width: 6, Height: 6},
		}},
	}
	require.NoError(t, dm.RegisterTemplate(template))

	require.Error(t, dm.CreateDashboard(context.Background(), "missing", "dash", nil))

	require.NoError(t, dm.CreateDashboard(context.Background(), "t1", "dash1", map[string]any{
		"service": "svc-override",
		"count":   3,
	}))

	require.Len(t, client.putInputs, 1)
	require.Equal(t, "dash1", aws.ToString(client.putInputs[0].DashboardName))

	var decoded map[string]any
	require.NoError(t, json.Unmarshal([]byte(aws.ToString(client.putInputs[0].DashboardBody)), &decoded))
	widgetsAny, ok := decoded["widgets"].([]any)
	require.True(t, ok)
	require.Len(t, widgetsAny, 1)

	widget, ok := widgetsAny[0].(map[string]any)
	require.True(t, ok)
	props, ok := widget["properties"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "svc-override-prod-3", props["title"])
	require.Equal(t, "us-east-1", props["region"])

	metricsAny, ok := props["metrics"].([]any)
	require.True(t, ok)
	require.Len(t, metricsAny, 1)
	metricRow, ok := metricsAny[0].([]any)
	require.True(t, ok)
	require.GreaterOrEqual(t, len(metricRow), 3)
	require.Equal(t, "ns", metricRow[0])
	require.Equal(t, "Requests", metricRow[1])

	// Dimension order is non-deterministic; normalize into a map.
	pairsEnd := len(metricRow)
	var options map[string]any
	if last, ok := metricRow[len(metricRow)-1].(map[string]any); ok {
		options = last
		pairsEnd--
	}
	dims := make(map[string]string)
	for i := 2; i+1 < pairsEnd; i += 2 {
		key, _ := metricRow[i].(string)
		val, _ := metricRow[i+1].(string)
		if key != "" {
			dims[key] = val
		}
	}
	require.Equal(t, map[string]string{
		"Service": "svc-override",
		"Env":     "prod",
	}, dims)
	require.NotNil(t, options)
	require.Equal(t, "Sum", options["stat"])
	require.Equal(t, float64(60), options["period"])
	require.Equal(t, "svc-override-prod", options["label"])

	// Update dashboard merges variables.
	require.Error(t, dm.UpdateDashboard(context.Background(), "missing", nil))
	require.NoError(t, dm.UpdateDashboard(context.Background(), "dash1", map[string]any{
		"count": 4,
	}))
	require.Len(t, client.putInputs, 2)

	deployed := dm.deployedDashboards["dash1"]
	require.Equal(t, "svc-override", deployed.Variables["service"])
	require.Equal(t, 4, deployed.Variables["count"])

	// Getters return copy maps.
	deployedCopy := dm.GetDeployedDashboards()
	require.Contains(t, deployedCopy, "dash1")
	deployedCopy["new"] = &DeployedDashboard{Name: "new"}
	require.NotContains(t, dm.deployedDashboards, "new")

	templatesCopy := dm.GetTemplates()
	require.Contains(t, templatesCopy, "t1")
	templatesCopy["new"] = &DashboardTemplate{ID: "new"}
	require.NotContains(t, dm.templates, "new")

	// GetDashboard forwards to client.
	client.getOutput = &cloudwatch.GetDashboardOutput{
		DashboardName: aws.String("dash1"),
	}
	out, err := dm.GetDashboard(context.Background(), "dash1")
	require.NoError(t, err)
	require.Equal(t, "dash1", aws.ToString(out.DashboardName))

	// Delete dashboard updates internal tracking.
	require.NoError(t, dm.DeleteDashboard(context.Background(), "dash1"))
	require.Len(t, client.deleteInputs, 1)
	require.Equal(t, DashboardStatusDeleted, dm.deployedDashboards["dash1"].Status)
}

func TestDashboardManager_ErrorPaths(t *testing.T) {
	t.Run("CreateDashboard fails when JSON marshal fails", func(t *testing.T) {
		client := &fakeCloudWatchDashboardClient{}
		dm := NewDashboardManager(client, DashboardManagerConfig{
			Namespace:   "ns",
			Environment: "prod",
			Region:      "us-east-1",
		})

		template := &DashboardTemplate{
			ID:      "t1",
			Version: "1",
			Widgets: []WidgetTemplate{{
				Type: "metric",
				Properties: map[string]any{
					"bad": func() {},
				},
			}},
		}
		require.NoError(t, dm.RegisterTemplate(template))

		require.Error(t, dm.CreateDashboard(context.Background(), "t1", "dash", nil))
	})

	t.Run("CreateDashboard returns client error", func(t *testing.T) {
		client := &fakeCloudWatchDashboardClient{putErr: errors.New("put failed")}
		dm := NewDashboardManager(client, DashboardManagerConfig{
			Namespace:   "ns",
			Environment: "prod",
			Region:      "us-east-1",
		})
		require.NoError(t, dm.RegisterTemplate(&DashboardTemplate{ID: "t1", Version: "1"}))
		require.Error(t, dm.CreateDashboard(context.Background(), "t1", "dash", nil))
	})

	t.Run("UpdateDashboard errors when template missing", func(t *testing.T) {
		client := &fakeCloudWatchDashboardClient{}
		dm := NewDashboardManager(client, DashboardManagerConfig{
			Namespace:   "ns",
			Environment: "prod",
			Region:      "us-east-1",
		})

		dm.deployedDashboards["dash"] = &DeployedDashboard{
			Name:       "dash",
			TemplateID: "missing",
			Variables:  map[string]any{"k": "v"},
		}

		require.Error(t, dm.UpdateDashboard(context.Background(), "dash", nil))
	})

	t.Run("UpdateDashboard returns client error", func(t *testing.T) {
		client := &fakeCloudWatchDashboardClient{}
		dm := NewDashboardManager(client, DashboardManagerConfig{
			Namespace:   "ns",
			Environment: "prod",
			Region:      "us-east-1",
		})
		require.NoError(t, dm.RegisterTemplate(&DashboardTemplate{ID: "t1", Version: "1"}))
		require.NoError(t, dm.CreateDashboard(context.Background(), "t1", "dash", nil))
		client.putErr = errors.New("put failed")
		require.Error(t, dm.UpdateDashboard(context.Background(), "dash", nil))
	})

	t.Run("DeleteDashboard returns client error", func(t *testing.T) {
		client := &fakeCloudWatchDashboardClient{deleteErr: errors.New("delete failed")}
		dm := NewDashboardManager(client, DashboardManagerConfig{
			Namespace:   "ns",
			Environment: "prod",
			Region:      "us-east-1",
		})

		require.Error(t, dm.DeleteDashboard(context.Background(), "dash"))
	})

	t.Run("GetDashboard returns client error", func(t *testing.T) {
		client := &fakeCloudWatchDashboardClient{getErr: errors.New("get failed")}
		dm := NewDashboardManager(client, DashboardManagerConfig{
			Namespace:   "ns",
			Environment: "prod",
			Region:      "us-east-1",
		})

		_, err := dm.GetDashboard(context.Background(), "dash")
		require.Error(t, err)
	})

	t.Run("ListDashboards returns client error", func(t *testing.T) {
		client := &fakeCloudWatchDashboardClient{listErr: errors.New("list failed")}
		dm := NewDashboardManager(client, DashboardManagerConfig{
			Namespace:   "ns",
			Environment: "prod",
			Region:      "us-east-1",
		})
		_, err := dm.ListDashboards(context.Background())
		require.Error(t, err)
	})
}

func TestDashboardManager_ListAndSyncAndAutoUpdate(t *testing.T) {
	now := time.Now()
	page1 := &cloudwatch.ListDashboardsOutput{
		DashboardEntries: []types.DashboardEntry{
			{DashboardName: aws.String("dash-a"), LastModified: aws.Time(now)},
		},
		NextToken: aws.String("t2"),
	}
	page2 := &cloudwatch.ListDashboardsOutput{
		DashboardEntries: []types.DashboardEntry{
			{DashboardName: aws.String("dash-b"), LastModified: aws.Time(now.Add(time.Minute))},
		},
	}

	client := &fakeCloudWatchDashboardClient{
		listPages: map[string]*cloudwatch.ListDashboardsOutput{
			"":   page1,
			"t2": page2,
		},
	}

	dm := NewDashboardManager(client, DashboardManagerConfig{
		Namespace:      "ns",
		Environment:    "prod",
		Region:         "us-east-1",
		UpdateInterval: 5 * time.Millisecond,
		AutoUpdate:     true,
	})

	// ListDashboards paginates.
	dashboards, err := dm.ListDashboards(context.Background())
	require.NoError(t, err)
	require.Len(t, dashboards, 2)
	require.Len(t, client.listTokens, 2)

	// SyncDashboards marks unknown as deleted and keeps existing active.
	dm.deployedDashboards["dash-a"] = &DeployedDashboard{Name: "dash-a", Status: DashboardStatusActive}
	dm.deployedDashboards["dash-missing"] = &DeployedDashboard{Name: "dash-missing", Status: DashboardStatusActive}

	require.NoError(t, dm.SyncDashboards(context.Background()))
	require.Equal(t, DashboardStatusActive, dm.deployedDashboards["dash-a"].Status)
	require.Equal(t, now, dm.deployedDashboards["dash-a"].LastUpdated)
	require.Equal(t, DashboardStatusDeleted, dm.deployedDashboards["dash-missing"].Status)

	// StartAutoUpdate runs a background sync until the context is cancelled.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client.listPages = map[string]*cloudwatch.ListDashboardsOutput{"": {DashboardEntries: nil}}
	dm.deployedDashboards["dash-a"].Status = DashboardStatusActive
	dm.StartAutoUpdate(ctx)

	require.Eventually(t, func() bool {
		return dm.deployedDashboards["dash-a"].Status == DashboardStatusDeleted && len(client.listTokens) >= 3
	}, time.Second, 10*time.Millisecond)

	cancel()
}
