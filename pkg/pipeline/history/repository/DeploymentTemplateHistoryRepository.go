package repository

import (
	"github.com/devtron-labs/devtron/pkg/sql"
	"github.com/go-pg/pg"
	"go.uber.org/zap"
	"time"
)

type DeploymentTemplateHistoryRepository interface {
	CreateHistory(chart *DeploymentTemplateHistory) (*DeploymentTemplateHistory, error)
	CreateHistoryWithTxn(chart *DeploymentTemplateHistory, tx *pg.Tx) (*DeploymentTemplateHistory, error)
	GetHistoryForDeployedCharts(pipelineId int) ([]*DeploymentTemplateHistory, error)
}

type DeploymentTemplateHistoryRepositoryImpl struct {
	dbConnection *pg.DB
	logger       *zap.SugaredLogger
}

func NewDeploymentTemplateHistoryRepositoryImpl(logger *zap.SugaredLogger, dbConnection *pg.DB) *DeploymentTemplateHistoryRepositoryImpl {
	return &DeploymentTemplateHistoryRepositoryImpl{dbConnection: dbConnection, logger: logger}
}

type DeploymentTemplateHistory struct {
	tableName               struct{}  `sql:"deployment_template_history" pg:",discard_unknown_columns"`
	Id                      int       `sql:"id,pk"`
	PipelineId              int       `sql:"pipeline_id, notnull"`
	ImageDescriptorTemplate string    `sql:"image_descriptor_template"`
	Template                string    `sql:"template"`
	TargetEnvironment       int       `sql:"target_environment"`
	TemplateName            string    `sql:"template_name"`
	TemplateVersion         string    `sql:"template_version"`
	IsAppMetricsEnabled     bool      `sql:"is_app_metrics_enabled"`
	Deployed                bool      `sql:"deployed"`
	DeployedOn              time.Time `sql:"deployed_on"`
	DeployedBy              int32     `sql:"deployed_by"`
	sql.AuditLog
}

func (impl DeploymentTemplateHistoryRepositoryImpl) CreateHistory(chart *DeploymentTemplateHistory) (*DeploymentTemplateHistory, error) {
	err := impl.dbConnection.Insert(chart)
	if err != nil {
		impl.logger.Errorw("err in creating deployment template history entry", "err", err, "history", chart)
		return chart, err
	}
	return chart, nil
}

func (impl DeploymentTemplateHistoryRepositoryImpl) CreateHistoryWithTxn(chart *DeploymentTemplateHistory, tx *pg.Tx) (*DeploymentTemplateHistory, error) {
	err := tx.Insert(chart)
	if err != nil {
		impl.logger.Errorw("err in creating deployment template history entry", "err", err, "history", chart)
		return chart, err
	}
	return chart, nil
}

func (impl DeploymentTemplateHistoryRepositoryImpl) GetHistoryForDeployedCharts(pipelineId int) ([]*DeploymentTemplateHistory, error) {
	var histories []*DeploymentTemplateHistory
	err := impl.dbConnection.Model(&histories).Where("pipeline_id = ?", pipelineId).
		Where("deployed = ?", true).Select()
	if err != nil {
		impl.logger.Errorw("error in getting deployment template history", "err", err)
		return histories, err
	}
	return histories, nil
}
