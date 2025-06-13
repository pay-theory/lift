package performance

import (
	"context"
	"fmt"
	"math"
	"runtime"
	"sync"
	"time"
)

// PerformanceOptimizer provides comprehensive performance monitoring and optimization
type PerformanceOptimizer struct {
	monitors        []PerformanceMonitor
	benchmarks      []Benchmark
	analyzers       []PerformanceAnalyzer
	optimizers      []Optimizer
	alerting        AlertingSystem
	trending        TrendAnalyzer
	recommendations RecommendationEngine
	config          PerformanceConfig
	mu              sync.RWMutex
}

// PerformanceConfig configures performance optimization behavior
type PerformanceConfig struct {
	MonitoringInterval  time.Duration
	BenchmarkTimeout    time.Duration
	RegressionThreshold float64
	AlertThresholds     AlertThresholds
	TrendAnalysisWindow time.Duration
	OptimizationLevel   OptimizationLevel
	EnablePredictive    bool
	EnableAutoOptimize  bool
}

type OptimizationLevel string

const (
	OptimizationLevelBasic      OptimizationLevel = "basic"
	OptimizationLevelStandard   OptimizationLevel = "standard"
	OptimizationLevelAggressive OptimizationLevel = "aggressive"
	OptimizationLevelExpert     OptimizationLevel = "expert"
)

type AlertThresholds struct {
	ResponseTime   time.Duration
	Throughput     float64
	ErrorRate      float64
	MemoryUsage    float64
	CPUUsage       float64
	DiskUsage      float64
	NetworkLatency time.Duration
}

// Core performance types
type PerformanceMetrics struct {
	Timestamp      time.Time
	ResponseTime   time.Duration
	Throughput     float64
	ErrorRate      float64
	MemoryUsage    MemoryMetrics
	CPUUsage       CPUMetrics
	DiskUsage      DiskMetrics
	NetworkMetrics NetworkMetrics
	CustomMetrics  map[string]float64
	RequestCount   int64
	ErrorCount     int64
	ActiveUsers    int64
}

type MemoryMetrics struct {
	Used        uint64
	Available   uint64
	Total       uint64
	Percentage  float64
	GCPauses    []time.Duration
	Allocations uint64
	Frees       uint64
}

type CPUMetrics struct {
	Usage       float64
	LoadAverage []float64
	Cores       int
	Frequency   float64
	Temperature float64
}

type DiskMetrics struct {
	ReadOps    uint64
	WriteOps   uint64
	ReadBytes  uint64
	WriteBytes uint64
	Usage      float64
	IOPS       float64
	Latency    time.Duration
}

type NetworkMetrics struct {
	BytesIn     uint64
	BytesOut    uint64
	PacketsIn   uint64
	PacketsOut  uint64
	Latency     time.Duration
	Bandwidth   float64
	Connections int
}

// Performance interfaces
type PerformanceMonitor interface {
	StartMonitoring(ctx context.Context, target string) error
	StopMonitoring() error
	GetMetrics(ctx context.Context) (PerformanceMetrics, error)
	DetectRegression(current, baseline PerformanceMetrics) bool
	GetThresholds() PerformanceThresholds
	SetThresholds(thresholds PerformanceThresholds) error
}

type PerformanceThresholds struct {
	MaxResponseTime   time.Duration
	MinThroughput     float64
	MaxErrorRate      float64
	MaxMemoryUsage    float64
	MaxCPUUsage       float64
	MaxDiskUsage      float64
	MaxNetworkLatency time.Duration
}

type Benchmark interface {
	Run(ctx context.Context, config BenchmarkConfig) (BenchmarkResult, error)
	GetBaseline() BenchmarkResult
	SetBaseline(baseline BenchmarkResult) error
	Compare(current, baseline BenchmarkResult) ComparisonResult
	GenerateReport() BenchmarkReport
}

type BenchmarkConfig struct {
	Duration         time.Duration
	Concurrency      int
	RequestRate      float64
	PayloadSize      int
	WarmupDuration   time.Duration
	CooldownDuration time.Duration
	Scenarios        []BenchmarkScenario
	Targets          []string
}

type BenchmarkScenario struct {
	Name        string
	Weight      float64
	Operations  []Operation
	Constraints []Constraint
}

type Operation struct {
	Type    OperationType
	Target  string
	Payload []byte
	Headers map[string]string
	Timeout time.Duration
	Retries int
}

type OperationType string

const (
	OperationTypeHTTPGet    OperationType = "http_get"
	OperationTypeHTTPPost   OperationType = "http_post"
	OperationTypeHTTPPut    OperationType = "http_put"
	OperationTypeHTTPDelete OperationType = "http_delete"
	OperationTypeDatabase   OperationType = "database"
	OperationTypeCache      OperationType = "cache"
	OperationTypeCompute    OperationType = "compute"
)

type Constraint struct {
	Type     ConstraintType
	Value    float64
	Operator ConstraintOperator
	Metric   string
}

type ConstraintType string

const (
	ConstraintTypeResponseTime ConstraintType = "response_time"
	ConstraintTypeThroughput   ConstraintType = "throughput"
	ConstraintTypeErrorRate    ConstraintType = "error_rate"
	ConstraintTypeMemory       ConstraintType = "memory"
	ConstraintTypeCPU          ConstraintType = "cpu"
)

type ConstraintOperator string

const (
	ConstraintOperatorLT  ConstraintOperator = "lt"
	ConstraintOperatorLTE ConstraintOperator = "lte"
	ConstraintOperatorGT  ConstraintOperator = "gt"
	ConstraintOperatorGTE ConstraintOperator = "gte"
	ConstraintOperatorEQ  ConstraintOperator = "eq"
)

type BenchmarkResult struct {
	Config         BenchmarkConfig
	StartTime      time.Time
	EndTime        time.Time
	Duration       time.Duration
	TotalRequests  int64
	SuccessfulReqs int64
	FailedRequests int64
	ResponseTimes  ResponseTimeStats
	Throughput     ThroughputStats
	ErrorStats     ErrorStats
	ResourceUsage  ResourceUsageStats
	Percentiles    PercentileStats
	CustomMetrics  map[string]interface{}
}

type ResponseTimeStats struct {
	Min    time.Duration
	Max    time.Duration
	Mean   time.Duration
	Median time.Duration
	StdDev time.Duration
	P95    time.Duration
	P99    time.Duration
	P999   time.Duration
}

type ThroughputStats struct {
	RequestsPerSecond float64
	BytesPerSecond    float64
	Peak              float64
	Average           float64
	Minimum           float64
}

type ErrorStats struct {
	TotalErrors   int64
	ErrorRate     float64
	ErrorTypes    map[string]int64
	TimeoutErrors int64
	NetworkErrors int64
	ServerErrors  int64
}

type ResourceUsageStats struct {
	Memory  MemoryUsageStats
	CPU     CPUUsageStats
	Disk    DiskUsageStats
	Network NetworkUsageStats
}

type MemoryUsageStats struct {
	Peak    uint64
	Average uint64
	Minimum uint64
	GCCount int
	GCTime  time.Duration
}

type CPUUsageStats struct {
	Peak    float64
	Average float64
	Minimum float64
	Cores   int
}

type DiskUsageStats struct {
	ReadBytes   uint64
	WriteBytes  uint64
	ReadOps     uint64
	WriteOps    uint64
	AverageIOPS float64
}

type NetworkUsageStats struct {
	BytesIn     uint64
	BytesOut    uint64
	PacketsIn   uint64
	PacketsOut  uint64
	Connections int
}

type PercentileStats struct {
	P50  time.Duration
	P75  time.Duration
	P90  time.Duration
	P95  time.Duration
	P99  time.Duration
	P999 time.Duration
}

type ComparisonResult struct {
	Baseline        BenchmarkResult
	Current         BenchmarkResult
	Improvements    []Improvement
	Regressions     []Regression
	OverallChange   PerformanceChange
	Significance    StatisticalSignificance
	Recommendations []string
}

type Improvement struct {
	Metric       string
	OldValue     float64
	NewValue     float64
	Change       float64
	Percentage   float64
	Significance float64
}

type Regression struct {
	Metric     string
	OldValue   float64
	NewValue   float64
	Change     float64
	Percentage float64
	Severity   RegressionSeverity
}

type RegressionSeverity string

const (
	RegressionSeverityMinor    RegressionSeverity = "minor"
	RegressionSeverityModerate RegressionSeverity = "moderate"
	RegressionSeverityMajor    RegressionSeverity = "major"
	RegressionSeverityCritical RegressionSeverity = "critical"
)

type PerformanceChange string

const (
	PerformanceChangeImproved  PerformanceChange = "improved"
	PerformanceChangeRegressed PerformanceChange = "regressed"
	PerformanceChangeUnchanged PerformanceChange = "unchanged"
	PerformanceChangeMixed     PerformanceChange = "mixed"
)

type StatisticalSignificance struct {
	PValue      float64
	Confidence  float64
	Significant bool
	TestType    string
}

type BenchmarkReport struct {
	Summary     BenchmarkSummary
	Results     []BenchmarkResult
	Comparisons []ComparisonResult
	Trends      TrendAnalysis
	Timestamp   time.Time
	Version     string
}

type BenchmarkSummary struct {
	TotalBenchmarks int
	Improvements    int
	Regressions     int
	Unchanged       int
	OverallScore    float64
}

type PerformanceAnalyzer interface {
	Analyze(ctx context.Context, metrics []PerformanceMetrics) (AnalysisResult, error)
	IdentifyBottlenecks(metrics PerformanceMetrics) []Bottleneck
	GenerateRecommendations(analysis AnalysisResult) []Recommendation
	PredictScaling(trends []PerformanceMetrics) ScalingPrediction
}

type AnalysisResult struct {
	Timestamp       time.Time
	AnalysisType    AnalysisType
	Bottlenecks     []Bottleneck
	Patterns        []PerformancePattern
	Anomalies       []Anomaly
	Trends          []Trend
	Predictions     []Prediction
	Score           PerformanceScore
	Recommendations []Recommendation
}

type AnalysisType string

const (
	AnalysisTypeRealTime    AnalysisType = "realtime"
	AnalysisTypeBatch       AnalysisType = "batch"
	AnalysisTypePredictive  AnalysisType = "predictive"
	AnalysisTypeComparative AnalysisType = "comparative"
)

type Bottleneck struct {
	Type        BottleneckType
	Component   string
	Severity    BottleneckSeverity
	Impact      ImpactLevel
	Description string
	Metrics     map[string]float64
	Suggestions []string
	Priority    int
}

type BottleneckType string

const (
	BottleneckTypeCPU      BottleneckType = "cpu"
	BottleneckTypeMemory   BottleneckType = "memory"
	BottleneckTypeDisk     BottleneckType = "disk"
	BottleneckTypeNetwork  BottleneckType = "network"
	BottleneckTypeDatabase BottleneckType = "database"
	BottleneckTypeCache    BottleneckType = "cache"
	BottleneckTypeAPI      BottleneckType = "api"
)

type BottleneckSeverity string

const (
	BottleneckSeverityLow      BottleneckSeverity = "low"
	BottleneckSeverityMedium   BottleneckSeverity = "medium"
	BottleneckSeverityHigh     BottleneckSeverity = "high"
	BottleneckSeverityCritical BottleneckSeverity = "critical"
)

type ImpactLevel string

const (
	ImpactLevelMinimal     ImpactLevel = "minimal"
	ImpactLevelLow         ImpactLevel = "low"
	ImpactLevelModerate    ImpactLevel = "moderate"
	ImpactLevelHigh        ImpactLevel = "high"
	ImpactLevelSignificant ImpactLevel = "significant"
)

type PerformancePattern struct {
	Type        PatternType
	Description string
	Frequency   time.Duration
	Confidence  float64
	Impact      ImpactLevel
	Examples    []PatternExample
}

type PatternType string

const (
	PatternTypeCyclic   PatternType = "cyclic"
	PatternTypeTrending PatternType = "trending"
	PatternTypeSpike    PatternType = "spike"
	PatternTypeDrop     PatternType = "drop"
	PatternTypeStable   PatternType = "stable"
	PatternTypeVolatile PatternType = "volatile"
)

type PatternExample struct {
	Timestamp time.Time
	Value     float64
	Context   string
}

type Anomaly struct {
	Type        AnomalyType
	Timestamp   time.Time
	Metric      string
	Expected    float64
	Actual      float64
	Deviation   float64
	Severity    AnomalySeverity
	Description string
	Causes      []string
}

type AnomalyType string

const (
	AnomalyTypeSpike     AnomalyType = "spike"
	AnomalyTypeDrop      AnomalyType = "drop"
	AnomalyTypeFlat      AnomalyType = "flat"
	AnomalyTypeOscillate AnomalyType = "oscillate"
	AnomalyTypeShift     AnomalyType = "shift"
)

type AnomalySeverity string

const (
	AnomalySeverityLow      AnomalySeverity = "low"
	AnomalySeverityMedium   AnomalySeverity = "medium"
	AnomalySeverityHigh     AnomalySeverity = "high"
	AnomalySeverityCritical AnomalySeverity = "critical"
)

type Trend struct {
	Metric     string
	Direction  TrendDirection
	Slope      float64
	Confidence float64
	Duration   time.Duration
	Projection TrendProjection
}

type TrendDirection string

const (
	TrendDirectionUp       TrendDirection = "up"
	TrendDirectionDown     TrendDirection = "down"
	TrendDirectionStable   TrendDirection = "stable"
	TrendDirectionVolatile TrendDirection = "volatile"
)

type TrendProjection struct {
	TimeHorizon time.Duration
	Values      []ProjectedValue
	Confidence  float64
}

type ProjectedValue struct {
	Timestamp  time.Time
	Value      float64
	LowerBound float64
	UpperBound float64
}

type Prediction struct {
	Type        PredictionType
	Metric      string
	TimeHorizon time.Duration
	Value       float64
	Confidence  float64
	Scenario    PredictionScenario
	Assumptions []string
}

type PredictionType string

const (
	PredictionTypeCapacity    PredictionType = "capacity"
	PredictionTypePerformance PredictionType = "performance"
	PredictionTypeFailure     PredictionType = "failure"
	PredictionTypeScaling     PredictionType = "scaling"
)

type PredictionScenario string

const (
	PredictionScenarioBest     PredictionScenario = "best"
	PredictionScenarioExpected PredictionScenario = "expected"
	PredictionScenarioWorst    PredictionScenario = "worst"
)

type PerformanceScore struct {
	Overall      float64
	ResponseTime float64
	Throughput   float64
	Reliability  float64
	Efficiency   float64
	Scalability  float64
	Breakdown    map[string]float64
}

type Recommendation struct {
	ID          string
	Type        RecommendationType
	Priority    RecommendationPriority
	Title       string
	Description string
	Impact      ImpactEstimate
	Effort      EffortEstimate
	Steps       []RecommendationStep
	References  []string
	Tags        []string
}

type RecommendationType string

const (
	RecommendationTypeOptimization   RecommendationType = "optimization"
	RecommendationTypeScaling        RecommendationType = "scaling"
	RecommendationTypeConfiguration  RecommendationType = "configuration"
	RecommendationTypeArchitecture   RecommendationType = "architecture"
	RecommendationTypeInfrastructure RecommendationType = "infrastructure"
)

type RecommendationPriority string

const (
	RecommendationPriorityLow      RecommendationPriority = "low"
	RecommendationPriorityMedium   RecommendationPriority = "medium"
	RecommendationPriorityHigh     RecommendationPriority = "high"
	RecommendationPriorityCritical RecommendationPriority = "critical"
)

type ImpactEstimate struct {
	Performance float64
	Cost        float64
	Risk        float64
	Timeline    time.Duration
	Confidence  float64
}

type EffortEstimate struct {
	Hours        float64
	Complexity   ComplexityLevel
	Skills       []string
	Resources    []string
	Dependencies []string
}

type ComplexityLevel string

const (
	ComplexityLevelLow    ComplexityLevel = "low"
	ComplexityLevelMedium ComplexityLevel = "medium"
	ComplexityLevelHigh   ComplexityLevel = "high"
	ComplexityLevelExpert ComplexityLevel = "expert"
)

type RecommendationStep struct {
	Order       int
	Description string
	Action      string
	Validation  string
	Rollback    string
}

type ScalingPrediction struct {
	CurrentCapacity CapacityMetrics
	PredictedDemand DemandForecast
	ScalingNeeds    ScalingRequirements
	Recommendations []ScalingRecommendation
	Timeline        ScalingTimeline
	CostEstimate    CostEstimate
}

type CapacityMetrics struct {
	CPU     CapacityInfo
	Memory  CapacityInfo
	Storage CapacityInfo
	Network CapacityInfo
}

type CapacityInfo struct {
	Current     float64
	Maximum     float64
	Utilization float64
	Available   float64
	Unit        string
}

type DemandForecast struct {
	TimeHorizon time.Duration
	Scenarios   []DemandScenario
	Confidence  float64
	Methodology string
}

type DemandScenario struct {
	Name        string
	Probability float64
	Growth      GrowthPattern
	Peak        PeakDemand
}

type GrowthPattern struct {
	Type        GrowthType
	Rate        float64
	Seasonality []SeasonalPattern
}

type GrowthType string

const (
	GrowthTypeLinear      GrowthType = "linear"
	GrowthTypeExponential GrowthType = "exponential"
	GrowthTypeLogarithmic GrowthType = "logarithmic"
	GrowthTypeSeasonal    GrowthType = "seasonal"
)

type SeasonalPattern struct {
	Period    time.Duration
	Amplitude float64
	Phase     time.Duration
}

type PeakDemand struct {
	Value     float64
	Timestamp time.Time
	Duration  time.Duration
	Frequency time.Duration
}

type ScalingRequirements struct {
	CPU     ScalingRequirement
	Memory  ScalingRequirement
	Storage ScalingRequirement
	Network ScalingRequirement
}

type ScalingRequirement struct {
	Current  float64
	Required float64
	Increase float64
	Timeline time.Duration
	Priority ScalingPriority
}

type ScalingPriority string

const (
	ScalingPriorityLow      ScalingPriority = "low"
	ScalingPriorityMedium   ScalingPriority = "medium"
	ScalingPriorityHigh     ScalingPriority = "high"
	ScalingPriorityCritical ScalingPriority = "critical"
)

type ScalingRecommendation struct {
	Type        ScalingType
	Description string
	Resources   []ResourceRecommendation
	Timeline    time.Duration
	Cost        float64
	Risk        RiskLevel
	Benefits    []string
}

type ScalingType string

const (
	ScalingTypeVertical   ScalingType = "vertical"
	ScalingTypeHorizontal ScalingType = "horizontal"
	ScalingTypeAuto       ScalingType = "auto"
	ScalingTypeElastic    ScalingType = "elastic"
)

type ResourceRecommendation struct {
	Type     ResourceType
	Current  ResourceSpec
	Proposed ResourceSpec
	Reason   string
}

type ResourceType string

const (
	ResourceTypeCPU      ResourceType = "cpu"
	ResourceTypeMemory   ResourceType = "memory"
	ResourceTypeStorage  ResourceType = "storage"
	ResourceTypeNetwork  ResourceType = "network"
	ResourceTypeInstance ResourceType = "instance"
)

type ResourceSpec struct {
	Value float64
	Unit  string
	Type  string
}

type RiskLevel string

const (
	RiskLevelLow      RiskLevel = "low"
	RiskLevelMedium   RiskLevel = "medium"
	RiskLevelHigh     RiskLevel = "high"
	RiskLevelCritical RiskLevel = "critical"
)

type ScalingTimeline struct {
	Phases []ScalingPhase
	Total  time.Duration
}

type ScalingPhase struct {
	Name         string
	Duration     time.Duration
	Actions      []string
	Milestones   []string
	Dependencies []string
}

type CostEstimate struct {
	Current   CostBreakdown
	Projected CostBreakdown
	Savings   CostBreakdown
	ROI       ROIAnalysis
}

type CostBreakdown struct {
	Compute float64
	Storage float64
	Network float64
	Other   float64
	Total   float64
	Period  time.Duration
}

type ROIAnalysis struct {
	Investment    float64
	Returns       float64
	PaybackPeriod time.Duration
	NPV           float64
	IRR           float64
}

// Optimizer interface for performance optimization
type Optimizer interface {
	Optimize(ctx context.Context, target OptimizationTarget) (OptimizationResult, error)
	GetOptimizationType() OptimizationType
	EstimateImpact(target OptimizationTarget) ImpactEstimate
	ValidateOptimization(result OptimizationResult) ValidationResult
}

type OptimizationTarget struct {
	Type        TargetType
	Component   string
	Metrics     PerformanceMetrics
	Constraints []OptimizationConstraint
	Objectives  []OptimizationObjective
}

type TargetType string

const (
	TargetTypeApplication    TargetType = "application"
	TargetTypeDatabase       TargetType = "database"
	TargetTypeCache          TargetType = "cache"
	TargetTypeNetwork        TargetType = "network"
	TargetTypeInfrastructure TargetType = "infrastructure"
)

type OptimizationConstraint struct {
	Type     ConstraintType
	Value    float64
	Operator ConstraintOperator
	Priority int
}

type OptimizationObjective struct {
	Metric   string
	Target   float64
	Weight   float64
	Priority int
}

type OptimizationType string

const (
	OptimizationTypeMemory        OptimizationType = "memory"
	OptimizationTypeCPU           OptimizationType = "cpu"
	OptimizationTypeIO            OptimizationType = "io"
	OptimizationTypeNetwork       OptimizationType = "network"
	OptimizationTypeAlgorithmic   OptimizationType = "algorithmic"
	OptimizationTypeArchitectural OptimizationType = "architectural"
)

type OptimizationResult struct {
	Target        OptimizationTarget
	Optimizations []AppliedOptimization
	BeforeMetrics PerformanceMetrics
	AfterMetrics  PerformanceMetrics
	Improvement   ImprovementMetrics
	Timestamp     time.Time
	Duration      time.Duration
	Success       bool
	Errors        []string
}

type AppliedOptimization struct {
	Type         OptimizationType
	Description  string
	Parameters   map[string]interface{}
	Impact       ImpactMetrics
	Reversible   bool
	RollbackInfo string
}

type ImpactMetrics struct {
	Performance PerformanceImpact
	Resource    ResourceImpact
	Cost        CostImpact
	Risk        RiskImpact
}

type PerformanceImpact struct {
	ResponseTime float64
	Throughput   float64
	ErrorRate    float64
	Availability float64
}

type ResourceImpact struct {
	CPU     float64
	Memory  float64
	Storage float64
	Network float64
}

type CostImpact struct {
	Operational    float64
	Infrastructure float64
	Development    float64
	Maintenance    float64
}

type RiskImpact struct {
	Stability   float64
	Security    float64
	Compliance  float64
	Operational float64
}

type ImprovementMetrics struct {
	ResponseTimeImprovement float64
	ThroughputImprovement   float64
	ErrorRateImprovement    float64
	ResourceEfficiency      float64
	CostReduction           float64
	OverallScore            float64
}

type ValidationResult struct {
	Valid       bool
	Score       float64
	Issues      []ValidationIssue
	Warnings    []string
	Suggestions []string
}

type ValidationIssue struct {
	Type        IssueType
	Severity    IssueSeverity
	Description string
	Impact      string
	Resolution  string
}

type IssueType string

const (
	IssueTypePerformance IssueType = "performance"
	IssueTypeStability   IssueType = "stability"
	IssueTypeSecurity    IssueType = "security"
	IssueTypeCompliance  IssueType = "compliance"
)

type IssueSeverity string

const (
	IssueSeverityInfo     IssueSeverity = "info"
	IssueSeverityWarning  IssueSeverity = "warning"
	IssueSeverityError    IssueSeverity = "error"
	IssueSeverityCritical IssueSeverity = "critical"
)

// AlertingSystem interface for performance alerts
type AlertingSystem interface {
	RegisterAlert(alert AlertRule) error
	TriggerAlert(ctx context.Context, alert Alert) error
	GetActiveAlerts() []Alert
	AcknowledgeAlert(alertID string) error
	ResolveAlert(alertID string) error
}

type AlertRule struct {
	ID          string
	Name        string
	Description string
	Metric      string
	Condition   AlertCondition
	Threshold   float64
	Duration    time.Duration
	Severity    AlertSeverity
	Actions     []AlertAction
	Enabled     bool
}

type AlertCondition string

const (
	AlertConditionGreater AlertCondition = "greater"
	AlertConditionLess    AlertCondition = "less"
	AlertConditionEqual   AlertCondition = "equal"
	AlertConditionChange  AlertCondition = "change"
)

type AlertSeverity string

const (
	AlertSeverityInfo     AlertSeverity = "info"
	AlertSeverityWarning  AlertSeverity = "warning"
	AlertSeverityError    AlertSeverity = "error"
	AlertSeverityCritical AlertSeverity = "critical"
)

type AlertAction struct {
	Type       ActionType
	Target     string
	Parameters map[string]interface{}
}

type ActionType string

const (
	ActionTypeEmail     ActionType = "email"
	ActionTypeSlack     ActionType = "slack"
	ActionTypeWebhook   ActionType = "webhook"
	ActionTypePagerDuty ActionType = "pagerduty"
	ActionTypeAutoScale ActionType = "autoscale"
	ActionTypeRestart   ActionType = "restart"
)

type Alert struct {
	ID          string
	RuleID      string
	Timestamp   time.Time
	Metric      string
	Value       float64
	Threshold   float64
	Severity    AlertSeverity
	Status      AlertStatus
	Description string
	Context     map[string]interface{}
}

type AlertStatus string

const (
	AlertStatusActive       AlertStatus = "active"
	AlertStatusAcknowledged AlertStatus = "acknowledged"
	AlertStatusResolved     AlertStatus = "resolved"
	AlertStatusSuppressed   AlertStatus = "suppressed"
)

// TrendAnalyzer interface for trend analysis
type TrendAnalyzer interface {
	AnalyzeTrends(ctx context.Context, metrics []PerformanceMetrics) (TrendAnalysis, error)
	PredictTrends(ctx context.Context, historical []PerformanceMetrics, horizon time.Duration) (TrendPrediction, error)
	DetectAnomalies(ctx context.Context, metrics []PerformanceMetrics) ([]Anomaly, error)
	GenerateTrendReport(analysis TrendAnalysis) TrendReport
}

type TrendAnalysis struct {
	Period     time.Duration
	Trends     []Trend
	Patterns   []PerformancePattern
	Anomalies  []Anomaly
	Summary    TrendSummary
	Confidence float64
	Timestamp  time.Time
}

type TrendSummary struct {
	OverallDirection TrendDirection
	Stability        float64
	Volatility       float64
	Seasonality      []SeasonalPattern
	KeyInsights      []string
}

type TrendPrediction struct {
	TimeHorizon time.Duration
	Predictions []Prediction
	Scenarios   []PredictionScenario
	Confidence  float64
	Methodology string
	Assumptions []string
}

type TrendReport struct {
	Analysis        TrendAnalysis
	Predictions     TrendPrediction
	Insights        []TrendInsight
	Recommendations []Recommendation
	Timestamp       time.Time
}

type TrendInsight struct {
	Type        InsightType
	Description string
	Impact      ImpactLevel
	Confidence  float64
	Evidence    []string
	Actions     []string
}

type InsightType string

const (
	InsightTypePerformance InsightType = "performance"
	InsightTypeCapacity    InsightType = "capacity"
	InsightTypeEfficiency  InsightType = "efficiency"
	InsightTypeReliability InsightType = "reliability"
	InsightTypeCost        InsightType = "cost"
)

// RecommendationEngine interface for generating optimization recommendations
type RecommendationEngine interface {
	GenerateRecommendations(ctx context.Context, analysis AnalysisResult) ([]Recommendation, error)
	PrioritizeRecommendations(recommendations []Recommendation) []Recommendation
	EstimateImpact(recommendation Recommendation) ImpactEstimate
	ValidateRecommendation(recommendation Recommendation) ValidationResult
}

// NewPerformanceOptimizer creates a new performance optimizer
func NewPerformanceOptimizer(config PerformanceConfig) *PerformanceOptimizer {
	return &PerformanceOptimizer{
		monitors:   make([]PerformanceMonitor, 0),
		benchmarks: make([]Benchmark, 0),
		analyzers:  make([]PerformanceAnalyzer, 0),
		optimizers: make([]Optimizer, 0),
		config:     config,
	}
}

// AddMonitor adds a performance monitor
func (po *PerformanceOptimizer) AddMonitor(monitor PerformanceMonitor) {
	po.mu.Lock()
	defer po.mu.Unlock()
	po.monitors = append(po.monitors, monitor)
}

// AddBenchmark adds a benchmark
func (po *PerformanceOptimizer) AddBenchmark(benchmark Benchmark) {
	po.mu.Lock()
	defer po.mu.Unlock()
	po.benchmarks = append(po.benchmarks, benchmark)
}

// AddAnalyzer adds a performance analyzer
func (po *PerformanceOptimizer) AddAnalyzer(analyzer PerformanceAnalyzer) {
	po.mu.Lock()
	defer po.mu.Unlock()
	po.analyzers = append(po.analyzers, analyzer)
}

// AddOptimizer adds an optimizer
func (po *PerformanceOptimizer) AddOptimizer(optimizer Optimizer) {
	po.mu.Lock()
	defer po.mu.Unlock()
	po.optimizers = append(po.optimizers, optimizer)
}

// OptimizePerformance performs comprehensive performance optimization
func (po *PerformanceOptimizer) OptimizePerformance(ctx context.Context, target string) (*PerformanceOptimizationResult, error) {
	result := &PerformanceOptimizationResult{
		Target:        target,
		StartTime:     time.Now(),
		Monitoring:    make(map[string]PerformanceMetrics),
		Benchmarks:    make(map[string]BenchmarkResult),
		Analysis:      make(map[string]AnalysisResult),
		Optimizations: make(map[string]OptimizationResult),
	}

	// Collect current performance metrics
	po.mu.RLock()
	monitors := po.monitors
	po.mu.RUnlock()

	for _, monitor := range monitors {
		metrics, err := monitor.GetMetrics(ctx)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Monitor failed: %v", err))
			continue
		}
		result.Monitoring[fmt.Sprintf("monitor_%d", len(result.Monitoring))] = metrics
	}

	// Run performance benchmarks
	po.mu.RLock()
	benchmarks := po.benchmarks
	po.mu.RUnlock()

	for _, benchmark := range benchmarks {
		benchConfig := BenchmarkConfig{
			Duration:    po.config.BenchmarkTimeout,
			Concurrency: runtime.NumCPU(),
			Targets:     []string{target},
		}

		benchResult, err := benchmark.Run(ctx, benchConfig)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Benchmark failed: %v", err))
			continue
		}
		result.Benchmarks[fmt.Sprintf("benchmark_%d", len(result.Benchmarks))] = benchResult
	}

	// Analyze performance data
	po.mu.RLock()
	analyzers := po.analyzers
	po.mu.RUnlock()

	var allMetrics []PerformanceMetrics
	for _, metrics := range result.Monitoring {
		allMetrics = append(allMetrics, metrics)
	}

	for _, analyzer := range analyzers {
		analysis, err := analyzer.Analyze(ctx, allMetrics)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Analysis failed: %v", err))
			continue
		}
		result.Analysis[fmt.Sprintf("analysis_%d", len(result.Analysis))] = analysis
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	// Calculate overall performance score
	result.PerformanceScore = po.calculatePerformanceScore(result)

	return result, nil
}

// calculatePerformanceScore calculates overall performance score
func (po *PerformanceOptimizer) calculatePerformanceScore(result *PerformanceOptimizationResult) PerformanceScore {
	score := PerformanceScore{
		Breakdown: make(map[string]float64),
	}

	// Calculate component scores
	var responseTimeScore, throughputScore, reliabilityScore, efficiencyScore, scalabilityScore float64
	var count int

	for _, metrics := range result.Monitoring {
		// Response time score (lower is better)
		if metrics.ResponseTime > 0 {
			responseTimeScore += math.Max(0, 100-float64(metrics.ResponseTime.Milliseconds()))
			count++
		}

		// Throughput score (higher is better)
		if metrics.Throughput > 0 {
			throughputScore += math.Min(100, metrics.Throughput)
		}

		// Reliability score (lower error rate is better)
		reliabilityScore += math.Max(0, 100-metrics.ErrorRate*100)

		// Efficiency score based on resource utilization
		cpuEfficiency := math.Max(0, 100-metrics.CPUUsage.Usage)
		memoryEfficiency := math.Max(0, 100-metrics.MemoryUsage.Percentage)
		efficiencyScore += (cpuEfficiency + memoryEfficiency) / 2
	}

	if count > 0 {
		responseTimeScore /= float64(count)
		throughputScore /= float64(count)
		reliabilityScore /= float64(count)
		efficiencyScore /= float64(count)
		scalabilityScore = 75.0 // Default scalability score
	}

	score.ResponseTime = responseTimeScore
	score.Throughput = throughputScore
	score.Reliability = reliabilityScore
	score.Efficiency = efficiencyScore
	score.Scalability = scalabilityScore

	// Calculate overall score as weighted average
	score.Overall = (responseTimeScore*0.25 + throughputScore*0.25 + reliabilityScore*0.25 + efficiencyScore*0.15 + scalabilityScore*0.10)

	score.Breakdown["response_time"] = responseTimeScore
	score.Breakdown["throughput"] = throughputScore
	score.Breakdown["reliability"] = reliabilityScore
	score.Breakdown["efficiency"] = efficiencyScore
	score.Breakdown["scalability"] = scalabilityScore

	return score
}

// PerformanceOptimizationResult contains comprehensive optimization results
type PerformanceOptimizationResult struct {
	Target           string
	StartTime        time.Time
	EndTime          time.Time
	Duration         time.Duration
	Monitoring       map[string]PerformanceMetrics
	Benchmarks       map[string]BenchmarkResult
	Analysis         map[string]AnalysisResult
	Optimizations    map[string]OptimizationResult
	PerformanceScore PerformanceScore
	Errors           []string
}
