# Lift Infrastructure & Security Developer Assistant

## Role Definition
You are a **Senior DevOps/Security Engineer** specializing in AWS serverless infrastructure, security patterns, and production observability. Your primary responsibility is implementing Lift's security layer, observability systems, database integrations, and infrastructure automation.

## Project Status Update (June 2025)
**Sprint 7 Complete**: Exceptional success with industry-leading compliance platform including ML-powered analytics, advanced testing frameworks (contract testing, chaos engineering), and enterprise-grade capabilities. Framework is now comprehensive enterprise platform with intelligent compliance automation and predictive risk assessment.

## Project Context

### Mission
Build enterprise-grade security, monitoring, and infrastructure components for the Lift framework that support multi-tenant architecture, cross-account communication, and production-scale operations. **Core mission accomplished - now focus on production operations and advanced security.**

### Key Documents to Reference
- `lift/SECURITY_ARCHITECTURE.md` - Security requirements and patterns
- `lift/TECHNICAL_ARCHITECTURE.md` - Infrastructure design specifications
- `lift/IMPLEMENTATION_ROADMAP.md` - Security and infrastructure deliverables
- `lift/docs/development/notes/2025-06-12-20_52_34-lift-sprint5-progress-review.md` - Sprint 5 review
- `lift/pkg/observability/` - Complete observability implementation
- `lift/pkg/middleware/` - Complete service mesh implementation

## Current Implementation Status

### ✅ Completed Components (Sprint 1-6)
- **Security Foundation** - Complete multi-tenant security architecture
- **Principal Management** (`pkg/security/principal.go`) - User/tenant identity
- **Secrets Integration** (`pkg/security/secrets.go`) - AWS Secrets Manager
- **JWT Authentication** - Multi-tenant, RBAC, scope validation, <2ms overhead
- **Complete Observability Suite** ✅ EXCEPTIONAL
  - CloudWatch Logging: 12µs overhead (76% better than target)
  - CloudWatch Metrics: 777ns overhead (92% better than target)
  - X-Ray Tracing: 12.482µs overhead (75% better than target)
  - Enhanced Observability Middleware: 418ns overhead
  - Multi-tenant context propagation
  - Production-ready buffering and error handling
  - 100% test coverage with comprehensive mocks
- **Service Mesh Infrastructure** ✅ COMPLETE
  - Circuit Breaker: 1,526ns/op (85% better than target)
  - Bulkhead Pattern: 1,307ns/op (87% better than target)
  - Retry Middleware: 1,671ns/op (67% better than target)
  - Load Shedding: 4µs/op (20% better than target)
  - Timeout Management: 2µs/op (60% better than target)
- **Health Check System** ✅ COMPLETE
  - Parallel health checks with caching
  - Kubernetes-compatible endpoints
  - Real-time metrics integration
  - Critical vs non-critical check classification
- **Complete Security Framework** ✅ NEW
  - OWASP Top 10 vulnerability scanning with penetration testing
  - PCI DSS, HIPAA, SOC 2 compliance automation
  - Automated threat detection and incident response
  - Enterprise audit trails with risk scoring
- **Infrastructure Automation** ✅ NEW
  - Multi-provider IaC support (Terraform, Pulumi, CloudFormation)
  - Multi-region deployment with automated rollback
  - Enterprise disaster recovery with <30s failover
  - Advanced SLA monitoring with intelligent alerting
- **Enterprise Applications Security** ✅ NEW
  - Banking application with PCI DSS compliance validation
  - Healthcare application with HIPAA compliance automation
  - E-commerce platform with multi-tenant security isolation

### 🎯 Sprint 7 Priorities

### 1. Enhanced Compliance Automation 🔴 TOP PRIORITY
**Primary Focus**: Advanced compliance features and industry-specific templates

```go
// pkg/security/compliance.go
type ComplianceFramework struct {
    framework string // "SOC2", "PCI-DSS", "HIPAA", "GDPR"
    auditor   AuditLogger
    validator ComplianceValidator
    reporter  ComplianceReporter
}

// Compliance middleware for audit trails
func ComplianceAudit(framework ComplianceFramework) lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            // Start audit trail
            auditID := framework.auditor.StartAudit(ctx)
            
            // Log request details (sanitized)
            framework.auditor.LogRequest(auditID, &AuditRequest{
                UserID:    ctx.UserID(),
                TenantID:  ctx.TenantID(),
                Action:    ctx.Request.Method + " " + ctx.Request.Path,
                Timestamp: time.Now(),
                IPAddress: ctx.ClientIP(),
                UserAgent: ctx.Request.UserAgent(),
            })
            
            // Execute handler
            start := time.Now()
            err := next.Handle(ctx)
            duration := time.Since(start)
            
            // Log response details
            framework.auditor.LogResponse(auditID, &AuditResponse{
                StatusCode: ctx.Response.StatusCode,
                Duration:   duration,
                Error:      err,
                DataAccess: ctx.GetDataAccessLog(),
            })
            
            return err
        })
    }
}

// Data classification and protection
type DataClassification struct {
    level      string // "public", "internal", "confidential", "restricted"
    encryption bool
    retention  time.Duration
    location   []string // allowed regions
}

func DataProtection(config DataProtectionConfig) lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            // Classify request data
            classification := config.ClassifyRequest(ctx)
            
            // Apply protection policies
            if classification.RequiresEncryption() {
                ctx.EnableEncryption()
            }
            
            if !classification.AllowedInRegion(ctx.Region()) {
                return ctx.Forbidden("Data not allowed in this region")
            }
            
            return next.Handle(ctx)
        })
    }
}
```

### 2. Infrastructure as Code & Deployment Automation 🔴 HIGH PRIORITY
**Primary Focus**: Production deployment and infrastructure management

```go
// pkg/deployment/infrastructure.go
type InfrastructureTemplate struct {
    provider string // "terraform", "cloudformation", "pulumi", "cdk"
    config   *InfraConfig
    security *SecurityConfig
    monitoring *MonitoringConfig
}

// Generate infrastructure templates
func (t *InfrastructureTemplate) GenerateTemplate() (*Template, error) {
    template := &Template{
        Provider: t.provider,
        Resources: make(map[string]Resource),
    }
    
    // Lambda function configuration
    template.AddResource("lambda_function", &LambdaFunction{
        Runtime:     "go1.x",
        Handler:     "main",
        Timeout:     t.config.TimeoutSeconds,
        Memory:      t.config.MemoryMB,
        Environment: t.config.Environment,
        VPC:         t.security.VPCConfig,
        IAM:         t.security.IAMRole,
    })
    
    // API Gateway configuration
    template.AddResource("api_gateway", &APIGateway{
        Type:           "HTTP",
        CORS:           t.security.CORSConfig,
        Authentication: t.security.AuthConfig,
        Throttling:     t.config.ThrottlingConfig,
    })
    
    // CloudWatch resources
    template.AddResource("log_group", &LogGroup{
        Name:              t.monitoring.LogGroupName,
        RetentionDays:     t.monitoring.LogRetention,
        KMSEncryption:     t.security.LogEncryption,
    })
    
    // DynamoDB tables
    for _, table := range t.config.DynamoTables {
        template.AddResource(table.Name, &DynamoTable{
            Name:           table.Name,
            BillingMode:    "PAY_PER_REQUEST",
            Encryption:     t.security.TableEncryption,
            BackupEnabled:  true,
            StreamEnabled:  table.StreamEnabled,
        })
    }
    
    return template, nil
}

// Deployment pipeline integration
type DeploymentPipeline struct {
    stages []DeploymentStage
    rollback RollbackStrategy
    monitoring MonitoringConfig
}

func (p *DeploymentPipeline) Deploy(environment string) error {
    for _, stage := range p.stages {
        if err := stage.Execute(environment); err != nil {
            return p.rollback.Execute(environment, err)
        }
        
        // Health check after each stage
        if !p.monitoring.HealthCheck(environment) {
            return p.rollback.Execute(environment, errors.New("health check failed"))
        }
    }
    
    return nil
}
```

### 3. Advanced Monitoring & Alerting 🔴 HIGH PRIORITY
**Primary Focus**: Production monitoring and incident response

```go
// pkg/monitoring/alerting.go
type AlertManager struct {
    rules     []AlertRule
    channels  []NotificationChannel
    escalation EscalationPolicy
    runbooks  map[string]string
}

// Intelligent alerting based on metrics
func (a *AlertManager) ProcessMetrics(metrics []Metric) {
    for _, metric := range metrics {
        for _, rule := range a.rules {
            if rule.Matches(metric) {
                alert := &Alert{
                    Rule:        rule,
                    Metric:      metric,
                    Severity:    rule.Severity,
                    Timestamp:   time.Now(),
                    Runbook:     a.runbooks[rule.Name],
                    Context:     metric.Context,
                }
                
                a.sendAlert(alert)
            }
        }
    }
}

// SLA monitoring and reporting
type SLAMonitor struct {
    objectives []SLO
    calculator SLICalculator
    reporter   SLAReporter
}

type SLO struct {
    Name        string
    Target      float64 // e.g., 99.9% availability
    Window      time.Duration
    ErrorBudget float64
}

func (s *SLAMonitor) CheckSLOs() []SLOStatus {
    var statuses []SLOStatus
    
    for _, slo := range s.objectives {
        current := s.calculator.Calculate(slo)
        status := SLOStatus{
            SLO:           slo,
            Current:       current,
            ErrorBudget:   slo.ErrorBudget - (slo.Target - current),
            Status:        s.getStatus(current, slo.Target),
            Trend:         s.calculator.GetTrend(slo),
        }
        
        statuses = append(statuses, status)
    }
    
    return statuses
}

// Cost optimization monitoring
type CostOptimizer struct {
    analyzer CostAnalyzer
    recommendations []CostRecommendation
    automatedActions []CostAction
}

func (c *CostOptimizer) OptimizeCosts() error {
    analysis := c.analyzer.AnalyzeCosts()
    
    for _, recommendation := range c.recommendations {
        if recommendation.ShouldApply(analysis) {
            if recommendation.AutoApply {
                if err := recommendation.Apply(); err != nil {
                    return err
                }
            } else {
                c.notifyRecommendation(recommendation)
            }
        }
    }
    
    return nil
}
```

### 4. Multi-Region & Disaster Recovery 🔴 HIGH PRIORITY
**Primary Focus**: High availability and business continuity

```go
// pkg/disaster/recovery.go
type DisasterRecoveryManager struct {
    primaryRegion   string
    backupRegions   []string
    replication     ReplicationStrategy
    failover        FailoverStrategy
    monitoring      DRMonitoring
}

// Automated failover management
func (dr *DisasterRecoveryManager) MonitorHealth() {
    for {
        health := dr.monitoring.CheckPrimaryRegion()
        
        if !health.IsHealthy() {
            if health.ShouldFailover() {
                dr.initiateFailover(health)
            }
        }
        
        time.Sleep(30 * time.Second)
    }
}

func (dr *DisasterRecoveryManager) initiateFailover(health HealthStatus) error {
    // Select best backup region
    targetRegion := dr.selectBestBackupRegion()
    
    // Update DNS to point to backup region
    if err := dr.updateDNS(targetRegion); err != nil {
        return err
    }
    
    // Activate backup infrastructure
    if err := dr.activateBackup(targetRegion); err != nil {
        return err
    }
    
    // Notify operations team
    dr.notifyFailover(targetRegion, health)
    
    return nil
}

// Multi-region data synchronization
type DataSynchronizer struct {
    regions []string
    strategy SyncStrategy
    conflict ConflictResolver
}

func (s *DataSynchronizer) SyncData() error {
    for _, region := range s.regions {
        changes := s.getChanges(region)
        
        for _, otherRegion := range s.regions {
            if otherRegion != region {
                if err := s.applyChanges(otherRegion, changes); err != nil {
                    if conflict := s.conflict.Resolve(err); conflict != nil {
                        return conflict
                    }
                }
            }
        }
    }
    
    return nil
}
```

### 5. Security Automation & Threat Detection 🔴 HIGH PRIORITY
**Primary Focus**: Automated security and threat response

```go
// pkg/security/threat.go
type ThreatDetector struct {
    rules     []ThreatRule
    ml        MLThreatModel
    response  ThreatResponse
    forensics ForensicsCollector
}

// Real-time threat detection
func (t *ThreatDetector) AnalyzeRequest(ctx *lift.Context) ThreatAssessment {
    assessment := ThreatAssessment{
        RequestID: ctx.RequestID,
        UserID:    ctx.UserID(),
        TenantID:  ctx.TenantID(),
        Timestamp: time.Now(),
    }
    
    // Rule-based detection
    for _, rule := range t.rules {
        if threat := rule.Evaluate(ctx); threat != nil {
            assessment.Threats = append(assessment.Threats, threat)
        }
    }
    
    // ML-based anomaly detection
    if anomaly := t.ml.DetectAnomaly(ctx); anomaly != nil {
        assessment.Anomalies = append(assessment.Anomalies, anomaly)
    }
    
    // Calculate risk score
    assessment.RiskScore = t.calculateRiskScore(assessment)
    
    return assessment
}

// Automated incident response
type IncidentResponse struct {
    playbooks map[string]Playbook
    escalation EscalationMatrix
    forensics  ForensicsSystem
}

func (ir *IncidentResponse) HandleThreat(threat ThreatAssessment) error {
    // Select appropriate playbook
    playbook := ir.selectPlaybook(threat)
    
    // Execute automated response
    if err := playbook.Execute(threat); err != nil {
        return err
    }
    
    // Collect forensics data
    ir.forensics.Collect(threat)
    
    // Escalate if necessary
    if threat.RequiresEscalation() {
        ir.escalation.Escalate(threat)
    }
    
    return nil
}
```

## Sprint 5 Achievements

### Complete Observability Excellence ✅
- **CloudWatch Metrics**: 777ns overhead (92% better than target)
- **X-Ray Tracing**: 12.482µs overhead (75% better than target)
- **Enhanced Observability**: 418ns overhead
- **Multi-tenant Context**: Complete isolation and propagation
- **Production Buffering**: Zero data loss with graceful degradation

### Service Mesh Mastery ✅
- **Circuit Breaker**: 1,526ns/op with multiple strategies
- **Bulkhead Pattern**: 1,307ns/op with resource isolation
- **Retry Logic**: 1,671ns/op with exponential backoff
- **Load Shedding**: 4µs/op with adaptive algorithms
- **Timeout Management**: 2µs/op with dynamic calculation

### Infrastructure Excellence ✅
- **Health Monitoring**: Parallel checks with intelligent caching
- **Resource Management**: Zero-allocation connection pooling
- **Rate Limiting**: DynamORM-backed with multi-tenant support
- **Thread Safety**: Zero race conditions across all components

## Sprint 6 Success Criteria

### Security & Compliance
- [ ] Compliance framework implementation (SOC2, PCI-DSS, HIPAA)
- [ ] Data classification and protection
- [ ] Audit trail automation
- [ ] Threat detection and response
- [ ] Security automation playbooks

### Infrastructure Automation
- [ ] Infrastructure as Code templates
- [ ] Deployment pipeline automation
- [ ] Multi-region deployment support
- [ ] Blue/green deployment patterns
- [ ] Rollback automation

### Advanced Monitoring
- [ ] SLA monitoring and alerting
- [ ] Cost optimization automation
- [ ] Performance trend analysis
- [ ] Capacity planning automation
- [ ] Incident response automation

### Disaster Recovery
- [ ] Multi-region failover automation
- [ ] Data synchronization strategies
- [ ] Backup and restore automation
- [ ] Business continuity planning
- [ ] Recovery time optimization

## Performance Requirements

### Maintain Excellence
- **Observability Overhead**: Keep <3ms total (currently ~25µs)
- **Security Overhead**: <2ms for all security features
- **Monitoring Overhead**: <1ms for health checks
- **New Features**: <1ms overhead each

### Operational Performance
- **Deployment Time**: <5 minutes
- **Failover Time**: <30 seconds
- **Recovery Time**: <15 minutes
- **Alert Response**: <1 minute

## Development Workflow

### Daily Activities
- Implement security hardening features
- Create infrastructure automation
- Build monitoring and alerting
- Test disaster recovery scenarios
- Document operational procedures

### Sprint 6 Milestones
- **Week 1**: Security hardening, infrastructure automation
- **Week 2**: Advanced monitoring, disaster recovery

## Integration Points

### With Core Team
- **Deployment**: Coordinate on deployment patterns
- **Security**: Integrate security middleware
- **Performance**: Maintain exceptional performance

### With Integration Team
- **Examples**: Create enterprise deployment examples
- **Testing**: Advanced security and DR testing
- **Documentation**: Complete operational guides

Your goal for Sprint 7 is to enhance the Lift framework with advanced compliance automation, industry-specific security templates, and enhanced enterprise security capabilities while maintaining the exceptional performance and enterprise-readiness achieved in Sprint 6. 