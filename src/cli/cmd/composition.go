package cmd

import (
	"sdd-cli/internal/infra"
	"sdd-cli/internal/usecases"
)

type Application struct {
	Init        *usecases.InitUseCase
	Adapters    AdaptersApplication
	Start       *usecases.StartWorkItemUseCase
	Status      *usecases.StatusUseCase
	Next        *usecases.NextUseCase
	Validate    *usecases.ValidateUseCase
	Begin       *usecases.BeginPhaseUseCase
	Deliver     *usecases.DeliverPhaseUseCase
	Approve     *usecases.ApproveUseCase
	Reject      *usecases.RejectUseCase
	Complete    *usecases.CompleteUseCase
	Archive     *usecases.ArchiveUseCase
	RecordEvent *usecases.RecordEventUseCase
}

func NewProductionApplication() Application {
	workItems := infra.NewFSWorkItemRepository()
	workflows := infra.NewFSWorkflowRepository()
	config := infra.NewFSConfigRepository()
	artifacts := infra.NewArtifactManager()
	adapters := infra.NewFSAdapterRepository()
	clock := infra.NewSystemClock()
	ids := infra.NewCryptoIDGenerator()
	validator := infra.NewFSValidationInspector()

	return Application{
		Init: usecases.NewInitUseCase(
			infra.NewFSProjectInitializer(),
		),
		Adapters: AdaptersApplication{
			List:    usecases.NewListAdaptersUseCase(adapters),
			Install: usecases.NewInstallAdapterUseCase(adapters),
		},
		Start: usecases.NewStartWorkItemUseCase(
			workItems,
			workflows,
			config,
			artifacts,
			clock,
			ids,
		),
		Status: usecases.NewStatusUseCase(workItems, workflows),
		Next:   usecases.NewNextUseCase(workItems, workflows),
		Validate: usecases.NewValidateUseCase(
			validator,
		),
		Begin: usecases.NewBeginPhaseUseCase(
			workItems,
			workflows,
			clock,
			ids,
		),
		Deliver: usecases.NewDeliverPhaseUseCase(
			workItems,
			workflows,
			artifacts,
			clock,
			ids,
		),
		Approve: usecases.NewApproveUseCase(
			workItems,
			workflows,
			artifacts,
			clock,
			ids,
		),
		Reject: usecases.NewRejectUseCase(
			workItems,
			workflows,
			clock,
			ids,
		),
		Complete: usecases.NewCompleteUseCase(
			workItems,
			workflows,
			artifacts,
			clock,
			ids,
		),
		Archive: usecases.NewArchiveUseCase(
			workItems,
			workflows,
			validator,
			workItems,
			clock,
			ids,
		),
		RecordEvent: usecases.NewRecordEventUseCase(
			workItems,
			clock,
			ids,
		),
	}
}

type AdaptersApplication struct {
	List    *usecases.ListAdaptersUseCase
	Install *usecases.InstallAdapterUseCase
}
