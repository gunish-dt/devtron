package history

import (
	repository2 "github.com/devtron-labs/devtron/internal/sql/repository"
	"github.com/devtron-labs/devtron/internal/sql/repository/chartConfig"
	"github.com/devtron-labs/devtron/internal/sql/repository/pipelineConfig"
	chartRepoRepository "github.com/devtron-labs/devtron/pkg/chartRepo/repository"
	"github.com/devtron-labs/devtron/pkg/pipeline/history/repository"
	"github.com/devtron-labs/devtron/pkg/sql"
	"github.com/go-pg/pg"
	"go.uber.org/zap"
	"time"
)

type DeploymentTemplateHistoryService interface {
	CreateDeploymentTemplateHistoryFromGlobalTemplate(chart *chartRepoRepository.Chart, tx *pg.Tx, IsAppMetricsEnabled bool) error
	CreateDeploymentTemplateHistoryFromEnvOverrideTemplate(envOverride *chartConfig.EnvConfigOverride, tx *pg.Tx, IsAppMetricsEnabled bool) error
	CreateDeploymentTemplateHistoryForDeploymentTrigger(pipeline *pipelineConfig.Pipeline, envOverride *chartConfig.EnvConfigOverride, renderedImageTemplate string, deployedOn time.Time, deployedBy int32) error
	GetHistoryForDeployedTemplates(pipelineId int) ([]*DeploymentTemplateHistoryDto, error)
}

type DeploymentTemplateHistoryServiceImpl struct {
	logger                              *zap.SugaredLogger
	deploymentTemplateHistoryRepository repository.DeploymentTemplateHistoryRepository
	pipelineRepository                  pipelineConfig.PipelineRepository
	chartRepository                     chartRepoRepository.ChartRepository
	chartRefRepository                  chartRepoRepository.ChartRefRepository
	envLevelAppMetricsRepository        repository2.EnvLevelAppMetricsRepository
	appLevelMetricsRepository           repository2.AppLevelMetricsRepository
}

func NewDeploymentTemplateHistoryServiceImpl(logger *zap.SugaredLogger, deploymentTemplateHistoryRepository repository.DeploymentTemplateHistoryRepository,
	pipelineRepository pipelineConfig.PipelineRepository,
	chartRepository chartRepoRepository.ChartRepository,
	chartRefRepository chartRepoRepository.ChartRefRepository,
	envLevelAppMetricsRepository repository2.EnvLevelAppMetricsRepository,
	appLevelMetricsRepository repository2.AppLevelMetricsRepository) *DeploymentTemplateHistoryServiceImpl {
	return &DeploymentTemplateHistoryServiceImpl{
		logger:                              logger,
		deploymentTemplateHistoryRepository: deploymentTemplateHistoryRepository,
		pipelineRepository:                  pipelineRepository,
		chartRepository:                     chartRepository,
		chartRefRepository:                  chartRefRepository,
		envLevelAppMetricsRepository:        envLevelAppMetricsRepository,
		appLevelMetricsRepository:           appLevelMetricsRepository,
	}
}

func (impl DeploymentTemplateHistoryServiceImpl) CreateDeploymentTemplateHistoryFromGlobalTemplate(chart *chartRepoRepository.Chart, tx *pg.Tx, IsAppMetricsEnabled bool) (err error) {
	//getting all pipelines without overridden charts
	pipelines, err := impl.pipelineRepository.FindAllPipelinesByChartsOverrideAndAppIdAndChartId(false, chart.AppId, chart.Id)
	if err != nil && err != pg.ErrNoRows {
		impl.logger.Errorw("err in getting pipelines, CreateDeploymentTemplateHistoryFromGlobalTemplate", "err", err, "chart", chart)
		return err
	}
	chartRef, err := impl.chartRefRepository.FindById(chart.ChartRefId)
	if err != nil {
		impl.logger.Errorw("err in getting chartRef, CreateDeploymentTemplateHistoryFromGlobalTemplate", "err", err, "chart", chart)
		return err
	}
	if len(chartRef.Name) == 0 {
		chartRef.Name = "Rollout Deployment"
	}
	for _, pipeline := range pipelines {
		historyModel := &repository.DeploymentTemplateHistory{
			PipelineId:              pipeline.Id,
			ImageDescriptorTemplate: chart.ImageDescriptorTemplate,
			Template:                chart.GlobalOverride,
			Deployed:                false,
			TemplateName:            chartRef.Name,
			TemplateVersion:         chartRef.Version,
			IsAppMetricsEnabled:     IsAppMetricsEnabled,
			AuditLog: sql.AuditLog{
				CreatedOn: chart.CreatedOn,
				CreatedBy: chart.CreatedBy,
				UpdatedOn: chart.UpdatedOn,
				UpdatedBy: chart.UpdatedBy,
			},
		}
		//creating new entry
		if tx != nil {
			_, err = impl.deploymentTemplateHistoryRepository.CreateHistoryWithTxn(historyModel, tx)
		} else {
			_, err = impl.deploymentTemplateHistoryRepository.CreateHistory(historyModel)
		}
		if err != nil {
			impl.logger.Errorw("err in creating history entry for deployment template", "err", err, "history", historyModel)
			return err
		}
	}
	return err
}

func (impl DeploymentTemplateHistoryServiceImpl) CreateDeploymentTemplateHistoryFromEnvOverrideTemplate(envOverride *chartConfig.EnvConfigOverride, tx *pg.Tx, IsAppMetricsEnabled bool) (err error) {
	//getting all pipelines without overridden charts
	pipelines, err := impl.pipelineRepository.GetByEnvOverrideId(envOverride.Id)
	if err != nil && err != pg.ErrNoRows {
		impl.logger.Errorw("err in getting pipelines, CreateDeploymentTemplateHistoryFromEnvOverrideTemplate", "err", err, "envOverrideId", envOverride.Id)
		return err
	}
	chart, err := impl.chartRepository.FindById(envOverride.ChartId)
	if err != nil {
		impl.logger.Errorw("err in getting global deployment template", "err", err, "chart", chart)
		return err
	}
	chartRef, err := impl.chartRefRepository.FindById(chart.ChartRefId)
	if err != nil {
		impl.logger.Errorw("err in getting chartRef, CreateDeploymentTemplateHistoryFromGlobalTemplate", "err", err, "chartRef", chartRef)
		return err
	}
	if len(chartRef.Name) == 0 {
		chartRef.Name = "Rollout Deployment"
	}
	for _, pipeline := range pipelines {
		historyModel := &repository.DeploymentTemplateHistory{
			PipelineId:              pipeline.Id,
			ImageDescriptorTemplate: chart.ImageDescriptorTemplate,
			Deployed:                false,
			TemplateName:            chartRef.Name,
			TemplateVersion:         chartRef.Version,
			IsAppMetricsEnabled:     IsAppMetricsEnabled,
			AuditLog: sql.AuditLog{
				CreatedOn: envOverride.CreatedOn,
				CreatedBy: envOverride.CreatedBy,
				UpdatedOn: envOverride.UpdatedOn,
				UpdatedBy: envOverride.UpdatedBy,
			},
		}
		if envOverride.IsOverride {
			historyModel.Template = envOverride.EnvOverrideValues
		} else {
			//this is for the case when env override is created for new cd pipelines with template = "{}"
			historyModel.Template = chart.GlobalOverride
		}
		//creating new entry
		if tx != nil {
			_, err = impl.deploymentTemplateHistoryRepository.CreateHistoryWithTxn(historyModel, tx)
		} else {
			_, err = impl.deploymentTemplateHistoryRepository.CreateHistory(historyModel)
		}
		if err != nil {
			impl.logger.Errorw("err in creating history entry for deployment template", "err", err, "history", historyModel)
			return err
		}
	}
	return err
}

func (impl DeploymentTemplateHistoryServiceImpl) CreateDeploymentTemplateHistoryForDeploymentTrigger(pipeline *pipelineConfig.Pipeline, envOverride *chartConfig.EnvConfigOverride, renderedImageTemplate string, deployedOn time.Time, deployedBy int32) error {
	chartRef, err := impl.chartRefRepository.FindById(envOverride.Chart.ChartRefId)
	if err != nil {
		impl.logger.Errorw("err in getting chartRef, CreateDeploymentTemplateHistoryFromGlobalTemplate", "err", err, "chartRef", chartRef)
		return err
	}
	if len(chartRef.Name) == 0 {
		chartRef.Name = "Rollout Deployment"
	}
	isAppMetricsEnabled := false
	envLevelAppMetrics, err := impl.envLevelAppMetricsRepository.FindByAppIdAndEnvId(pipeline.AppId, pipeline.EnvironmentId)
	if err != nil && err != pg.ErrNoRows {
		impl.logger.Errorw("error in getting env level app metrics", "err", err, "appId", pipeline.AppId, "envId", pipeline.EnvironmentId)
		return err
	} else if err == pg.ErrNoRows {
		appLevelAppMetrics, err := impl.appLevelMetricsRepository.FindByAppId(pipeline.AppId)
		if err != nil {
			impl.logger.Errorw("error in getting app level app metrics", "err", err, "appId", pipeline.AppId)
			return err
		}
		isAppMetricsEnabled = appLevelAppMetrics.AppMetrics
	} else {
		isAppMetricsEnabled = *envLevelAppMetrics.AppMetrics
	}
	historyModel := &repository.DeploymentTemplateHistory{
		PipelineId:              pipeline.Id,
		ImageDescriptorTemplate: renderedImageTemplate,
		Deployed:                true,
		DeployedBy:              deployedBy,
		DeployedOn:              deployedOn,
		TemplateName:            chartRef.Name,
		TemplateVersion:         chartRef.Version,
		IsAppMetricsEnabled:     isAppMetricsEnabled,
		AuditLog: sql.AuditLog{
			CreatedOn: deployedOn,
			CreatedBy: deployedBy,
			UpdatedOn: deployedOn,
			UpdatedBy: deployedBy,
		},
	}
	if envOverride.IsOverride {
		historyModel.Template = envOverride.EnvOverrideValues
	} else {
		historyModel.Template = envOverride.Chart.GlobalOverride
	}
	//creating new entry
	_, err = impl.deploymentTemplateHistoryRepository.CreateHistory(historyModel)
	if err != nil {
		impl.logger.Errorw("err in creating history entry for deployment template", "err", err, "history", historyModel)
		return err
	}
	return nil
}

func (impl DeploymentTemplateHistoryServiceImpl) GetHistoryForDeployedTemplates(pipelineId int) ([]*DeploymentTemplateHistoryDto, error) {
	histories, err := impl.deploymentTemplateHistoryRepository.GetHistoryForDeployedCharts(pipelineId)
	if err != nil {
		impl.logger.Errorw("error in getting deployment template history", "err", err, "pipelineId", pipelineId)
		return nil, err
	}
	var historiesDto []*DeploymentTemplateHistoryDto
	for _, history := range histories {
		historyDto := &DeploymentTemplateHistoryDto{
			Id:                      history.Id,
			PipelineId:              history.PipelineId,
			ImageDescriptorTemplate: history.ImageDescriptorTemplate,
			Template:                history.Template,
			TemplateVersion:         history.TemplateVersion,
			TemplateName:            history.TemplateName,
			TargetEnvironment:       history.TargetEnvironment,
			Deployed:                history.Deployed,
			DeployedOn:              history.DeployedOn,
			DeployedBy:              history.DeployedBy,
			AuditLog: sql.AuditLog{
				CreatedBy: history.CreatedBy,
				CreatedOn: history.CreatedOn,
				UpdatedBy: history.UpdatedBy,
				UpdatedOn: history.UpdatedOn,
			},
		}
		historiesDto = append(historiesDto, historyDto)
	}
	return historiesDto, nil
}
