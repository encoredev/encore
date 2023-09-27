package shutdown

import "context"

// Progress provides progress information about an ongoing graceful shutdown process.
//
// The process broadly consists of two phases:
//
// 1. Drain active tasks
//
// As soon as the graceful shutdown process is initiated, the service will stop accepting new
// incoming API calls and Pub/Sub messages. It will continue to process already running tasks
// until they complete (or the ForceCloseTasks deadline is reached).
//
// Additionally, all service structs that implement [Handler] will have their [Handler.Shutdown]
// function called when this phase begins. The [Handler.Shutdown] method receives a [Progress]
// struct that can be used to monitor the progress of the shutdown process, and allows the service
// to perform any necessary cleanup at the right time.
//
// This phase continues until all active tasks and handlers have completed or the ForceCloseTasks deadline
// is reached, whichever happens first. The OutstandingRequests, OutstandingPubSubMessages, and
// OutstandingTasks contexts provide insight into what tasks are still active.
//
// 2. Shut down infrastructure resources
//
// When all active tasks and [Handler.Shutdown] calls have completed, Encore begins shutting down
// infrastructure resources. Encore automatically closes all open database connections, cache connections,
// Pub/Sub connections, and other infrastructure resources.
//
// This phase continues until all infrastructure resources have been closed or the ForceShutdown deadline
// is reached, whichever happens first.
//
// 3. Exit
//
// Once phase two has completed, the process will exit.
// The exit code is 0 if the graceful shutdown completed successfully (meaning all resources
// returned before the exit deadline), or 1 otherwise.
type Progress struct {
	// OutstandingRequests is canceled when the service is no longer processing any incoming API calls.
	OutstandingRequests context.Context

	// OutstandingPubSubMessages is canceled when the service is no longer processing any Pub/Sub messages.
	OutstandingPubSubMessages context.Context

	// ForceCloseTasks is canceled when the graceful shutdown deadline is reached and it's time to
	// forcibly close active tasks (outstanding incoming API requests and Pub/Sub subscription messages).
	//
	// When ForceCloseTasks is closed, the contexts for all outstanding tasks are canceled.
	//
	// It is canceled early if all active tasks are done.
	ForceCloseTasks context.Context

	// OutstandingTasks is canceled when the service is no longer actively processing any tasks,
	// which includes both incoming API calls and Pub/Sub messages.
	//
	// It is canceled as soon as both OutstandingRequests and OutstandingPubSubMessages have been canceled.
	OutstandingTasks context.Context

	// ForceShutdown is closed when the graceful shutdown window has closed and it's time to
	// forcefully shut down.
	//
	// If the graceful shutdown window lapses before the cooperative shutdown is complete,
	// the ForceShutdown channel may be closed before RunningHandlers is canceled.
	//
	// It is canceled early if all running tasks have completed, all infrastructure resources are closed,
	// and all registered service Handler.Shutdown methods have returned.
	ForceShutdown context.Context
}

// Handler is the interface for resources that participate in the graceful shutdown process.
type Handler interface {
	// Shutdown is called by Encore when the graceful shutdown process is initiated.
	//
	// The provided Progress struct provides information about the graceful shutdown progress,
	// which can be used to determine at what point in time it's appropriate to close certain resources.
	//
	// For example, a service struct may want to wait for all incoming requests to complete
	// before it closes its client to a third-party service:
	//
	// 			func (s *MyService) Shutdown(p *shutdown.Progress) error {
	//				<-p.OutstandingRequests.Done()
	//				return s.client.Close()
	//			}
	//
	// The shutdown process is cooperative (to the extent it is possible),
	// and Encore will wait for all Handlers to return before closing
	// infrastructure resources and exiting the process,
	// until the ForceShutdown deadline is reached.
	//
	// The return value of Shutdown is used to report shutdown errors only,
	// and has no effect on the shutdown process.
	Shutdown(Progress) error
}
