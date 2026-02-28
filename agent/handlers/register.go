package handlers

import "github.com/typedduck/nestor/agent/executor"

// RegisterAll registers every available action handler with engine.
// Used by the remote agent, which targets fully-managed Linux systems.
func RegisterAll(engine *executor.Engine) {
	RegisterLocal(engine)
	engine.RegisterHandler("service.start", NewServiceStartHandler())
	engine.RegisterHandler("service.stop", NewServiceStopHandler())
	engine.RegisterHandler("service.reload", NewServiceReloadHandler())
	engine.RegisterHandler("service.restart", NewServiceRestartHandler())
}

// RegisterLocal registers the handlers appropriate for local execution.
// Service handlers are excluded: local playbooks target the developer's
// own machine where service management is not part of the workflow.
func RegisterLocal(engine *executor.Engine) {
	engine.RegisterHandler("package.install", NewPackageInstallHandler())
	engine.RegisterHandler("package.remove", NewPackageRemoveHandler())
	engine.RegisterHandler("package.update", NewPackageUpdateHandler())
	engine.RegisterHandler("package.upgrade", NewPackageUpgradeHandler())
	engine.RegisterHandler("file.content", NewFileContentHandler())
	engine.RegisterHandler("file.template", NewFileTemplateHandler())
	engine.RegisterHandler("file.upload", NewFileUploadHandler())
	engine.RegisterHandler("file.symlink", NewSymlinkHandler())
	engine.RegisterHandler("directory.create", NewDirectoryCreateHandler())
	engine.RegisterHandler("file.remove", NewFileRemoveHandler())
	engine.RegisterHandler("command.execute", NewCommandExecuteHandler())
	engine.RegisterHandler("script.execute", NewScriptExecuteHandler())
}
