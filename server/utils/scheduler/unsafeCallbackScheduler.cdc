import FlowToken from 0x0ae53cb6e3f42a79

// UnsafeCallbackScheduler is a simple implementation of the CallbackScheduler interface.
//
// WARNING:
// It is unsafe because it does not implement safe scheduling logic and it only simulates priority.
// It is used on emulator for testing purposes.
// The APIs simulate the behaviour you can expect from production implementation.
access(all) contract UnsafeCallbackScheduler {

    // Entitlements
    access(all) entitlement mayExecuteCallback
    access(all) entitlement mayCancelCallback
    access(all) entitlement mayReadCallbackStatus

    // Events
    access(all) event CallbackScheduled(ID: UInt64, timestamp: UFix64?, priority: UInt8, executionEffort: UInt64)
    access(all) event CallbackProcessed(ID: UInt64, executionEffort: UInt64)
    access(all) event CallbackExecuted(ID: UInt64)
    access(all) event CallbackCanceled(ID: UInt64)

    // Interfaces
    access(all) struct interface CallbackHandler {
        access(mayExecuteCallback) fun executeCallback(data: AnyStruct?)
    }

    access(all) resource interface Scheduler {
        access(all) fun schedule(
            callback: Capability<auth(mayExecuteCallback) &{CallbackHandler}>,
            data: AnyStruct?,
            timestamp: UFix64,
            priority: Priority,
            executionEffort: UInt64,
            fees: @FlowToken.Vault
        ): ScheduledCallback

        access(all) fun estimate(
            data: AnyStruct?,
            timestamp: UFix64,
            priority: Priority,
            executionEffort: UInt64
        ): EstimatedCallback?

        access(mayReadCallbackStatus) fun getStatus(ID: UInt64): Status
        access(mayCancelCallback) fun cancel(ID: UInt64): @FlowToken.Vault
        
        // service methods
        access(all) fun process()
        access(all) fun executeCallback(ID: UInt64)

        // helper methods
        access(all) fun printCallbackQueue()
    }

    // Enums
    access(all) enum Priority: UInt8 {
        access(all) case High
        access(all) case Medium
        access(all) case Low
    }

    access(all) enum Status: UInt8 {
        access(all) case Scheduled
        access(all) case Processed
        access(all) case Executed
        access(all) case Canceled
    }

    // Structs
    access(all) struct ScheduledCallback {
        access(self) let scheduler: Capability<auth(mayCancelCallback, mayReadCallbackStatus) &{Scheduler}>
        access(all) let ID: UInt64
        access(all) let timestamp: UFix64?

        access(all) fun status(): Status {
            return self.scheduler.borrow()!.getStatus(ID: self.ID)
        }

        access(all) fun cancel(): @FlowToken.Vault {
            return <-self.scheduler.borrow()!.cancel(ID: self.ID)
        }

        access(contract) init(
            scheduler: Capability<auth(mayCancelCallback, mayReadCallbackStatus) &{Scheduler}>, 
            ID: UInt64, 
            timestamp: UFix64?
        ) {
            self.scheduler = scheduler
            self.ID = ID
            self.timestamp = timestamp
        }
    }

    access(all) struct EstimatedCallback {
        access(all) let flowFee: UFix64
        access(all) let timestamp: UFix64

        access(contract) init(flowFee: UFix64, timestamp: UFix64) {
            self.flowFee = flowFee
            self.timestamp = timestamp
        }
    }

    // Resources
    access(all) resource CallbackData {
        access(all) let handler: Capability<auth(mayExecuteCallback) &{CallbackHandler}>
        access(all) let data: AnyStruct?
        access(all) let originalTimestamp: UFix64
        access(all) let priority: Priority
        access(all) let executionEffort: UInt64
        access(all) let fees: @FlowToken.Vault
        access(all) var status: Status
        access(all) let scheduledTimestamp: UFix64

        access(contract) init(
            handler: Capability<auth(mayExecuteCallback) &{CallbackHandler}>,
            data: AnyStruct?,
            originalTimestamp: UFix64,
            priority: Priority,
            executionEffort: UInt64,
            fees: @FlowToken.Vault,
            scheduledTimestamp: UFix64
        ) {
            self.handler = handler
            self.data = data
            self.originalTimestamp = originalTimestamp
            self.priority = priority
            self.executionEffort = executionEffort
            self.fees <- fees
            self.status = Status.Scheduled
            self.scheduledTimestamp = scheduledTimestamp
        }

        access(all) fun setStatus(newStatus: Status) {
            self.status = newStatus
        }

        access(contract) fun toString(): String {
            return "callback (status: ".concat(self.status.rawValue.toString())
                .concat(", timestamp: ").concat(self.scheduledTimestamp.toString())
                .concat(", priority: ").concat(self.priority.rawValue.toString())
                .concat(", executionEffort: ").concat(self.executionEffort.toString())
                .concat(", fees: ").concat(self.fees.balance.toString())
                .concat(")")
        }
    }

    access(all) resource UnsafeScheduler: Scheduler {
        access(self) var nextID: UInt64
        access(self) var callbacks: @{UInt64: CallbackData}
        access(self) var timestampQueue: {UFix64: [UInt64]}
        access(self) let HIGH_MULTIPLIER: UFix64
        access(self) let MEDIUM_MULTIPLIER: UFix64
        access(self) let LOW_MULTIPLIER: UFix64

        access(all) init() {
            self.nextID = 0
            self.callbacks <- {}
            self.timestampQueue = {}
            self.HIGH_MULTIPLIER = 10.0
            self.MEDIUM_MULTIPLIER = 5.0
            self.LOW_MULTIPLIER = 2.0
        }

        access(self) fun calculateScheduledTimestamp(timestamp: UFix64, priority: Priority): UFix64 {
            switch priority {
                case Priority.High:
                    return timestamp
                case Priority.Medium:
                    return timestamp + 5.0
                case Priority.Low:
                    let random = revertibleRandom<UInt8>(modulo: 100)
                    return timestamp + UFix64(random)
                default:
                    panic("Invalid priority ".concat(priority.rawValue.toString()))
            }
        }

        access(self) fun calculateFee(executionEffort: UInt64, priority: Priority): UFix64 {
            let baseFee = UFix64(executionEffort) / 10000.0
            switch priority {
                case Priority.High:
                    return baseFee * self.HIGH_MULTIPLIER
                case Priority.Medium:
                    return baseFee * self.MEDIUM_MULTIPLIER
                case Priority.Low:
                    return baseFee * self.LOW_MULTIPLIER
                default:
                    panic("Invalid priority ".concat(priority.rawValue.toString()))
            }
        }

        access(all) fun schedule(
            callback: Capability<auth(mayExecuteCallback) &{CallbackHandler}>,
            data: AnyStruct?,
            timestamp: UFix64,
            priority: Priority,
            executionEffort: UInt64,
            fees: @FlowToken.Vault
        ): ScheduledCallback {
            if timestamp <= getCurrentBlock().timestamp {
                panic("Timestamp ".concat(timestamp.toString()).concat(" is in the past, current timestamp is ".concat(getCurrentBlock().timestamp.toString())))
            }

            let requiredFee = self.calculateFee(executionEffort: executionEffort, priority: priority)
            if fees.balance < requiredFee {
                panic("Insufficient fees ".concat(requiredFee.toString()).concat(" required, but only ".concat(fees.balance.toString()).concat(" available")))
            }

            var scheduledTimestamp: UFix64? = self.calculateScheduledTimestamp(timestamp: timestamp, priority: priority)
            let callbackID = self.nextID
            self.nextID = self.nextID + 1

            self.callbacks[callbackID] <-! create CallbackData(
                handler: callback,
                data: data,
                originalTimestamp: timestamp,
                priority: priority,
                executionEffort: executionEffort,
                fees: <- fees,
                scheduledTimestamp: scheduledTimestamp!
            )

            if self.timestampQueue[scheduledTimestamp!] == nil {
                self.timestampQueue[scheduledTimestamp!] = []
            }
            self.timestampQueue[scheduledTimestamp!]!.append(callbackID)

            if priority == Priority.Low {
                scheduledTimestamp = nil
            }

            emit CallbackScheduled(
                ID: callbackID,
                timestamp: scheduledTimestamp,
                priority: priority.rawValue,
                executionEffort: executionEffort
            )

            return ScheduledCallback(
                scheduler: UnsafeCallbackScheduler.sharedScheduler, 
                ID: callbackID, 
                timestamp: scheduledTimestamp
            )
        }

        access(all) fun estimate(
            data: AnyStruct?,
            timestamp: UFix64,
            priority: Priority,
            executionEffort: UInt64
        ): EstimatedCallback? {
            if priority == Priority.Low {
                log("Low priority callbacks are not supported")
                return nil
            }
            if timestamp <= getCurrentBlock().timestamp {
                log("Timestamp is in the past")
                return nil
            }

            let fee = self.calculateFee(executionEffort: executionEffort, priority: priority)
            let scheduledTimestamp = self.calculateScheduledTimestamp(timestamp: timestamp, priority: priority)
            return EstimatedCallback(flowFee: fee, timestamp: scheduledTimestamp)
        }

        access(mayReadCallbackStatus) fun getStatus(ID: UInt64): Status {
            // TODO: keep status in seperate map to retain it after the callback is executed and garbage collected
            if let callback = &self.callbacks[ID] as &CallbackData? {
                return callback.status
            }
            panic("Callback not found")
        }

        access(mayCancelCallback) fun cancel(ID: UInt64): @FlowToken.Vault {
            let callback <- self.callbacks.remove(key: ID) ?? panic("Callback not found")
            self.timestampQueue[callback.scheduledTimestamp]?.remove(at: ID)

            let halfAmount = callback.fees.balance / 2.0
            let refund <- callback.fees.withdraw(amount: halfAmount) as! @FlowToken.Vault
            destroy callback

            emit CallbackCanceled(ID: ID)
            return <-refund
        }

        access(all) fun process() {
            self.printCallbackQueue()
            
            let currentTimestamp = getCurrentBlock().timestamp
            let timestampShouldBeProcessed = view fun (element: UFix64): Bool {
                return element <= currentTimestamp
            }
            let eligibleTimestamps = self.timestampQueue.keys.filter(timestampShouldBeProcessed)

            for timestamp in eligibleTimestamps {
                if let callbackIDs = self.timestampQueue[timestamp] {
                    for callbackID in callbackIDs {
                        let callback = &self.callbacks[callbackID] as &CallbackData?
                        if callback != nil && callback!.status == Status.Scheduled {
                            callback!.setStatus(newStatus: Status.Processed)
                            emit CallbackProcessed(ID: callbackID, executionEffort: callback!.executionEffort)
                        }
                    }
                }
            }
        }

        access(all) fun executeCallback(ID: UInt64) {
            let callback = &self.callbacks[ID] as &CallbackData?
            if callback == nil {
                panic("Callback not found")
            }
            if callback!.status != Status.Processed {
                panic("Callback not processed")
            }

            let handlerRef = callback!.handler.borrow() ?? panic("Cannot borrow callback handler")
            handlerRef.executeCallback(data: callback!.data)

            callback!.setStatus(newStatus: Status.Executed)
            emit CallbackExecuted(ID: ID)
        }
        
        // Helper function to print the callback queue
        access(all) fun printCallbackQueue() {
            log("[scheduler.queue] queue state at timestamp ".concat(getCurrentBlock().timestamp.toString()))
            for timestamp in self.timestampQueue.keys {
                log("- timestamp: ".concat(timestamp.toString()))

                if let callbackIDs = self.timestampQueue[timestamp] {
                    for callbackID in callbackIDs {
                        let callback = &self.callbacks[callbackID] as &CallbackData?
                        if callback != nil {
                            log("  | callback: ".concat(callbackID.toString()).concat(" status: ").concat(self.statusToString(status: callback!.status)).concat(" priority: ").concat(self.priorityToString(priority: callback!.priority)))
                        }
                    }
                }
            }
        }

        access(self) fun statusToString(status: Status): String {
            switch status {
                case Status.Scheduled:
                    return "scheduled"
                case Status.Processed:
                    return "processed"
                case Status.Executed:
                    return "executed"
                case Status.Canceled:
                    return "canceled"
                default:
                    panic("Invalid status ".concat(status.rawValue.toString()))
            }
        }

        access(self) fun priorityToString(priority: Priority): String {
            switch priority {
                case Priority.High:
                    return "high"
                case Priority.Medium:
                    return "medium"
                case Priority.Low:
                    return "low"
                default:
                    panic("Invalid priority ".concat(priority.rawValue.toString()))
            }
        }
    }

    access(self) var sharedScheduler: Capability<auth(mayCancelCallback, mayReadCallbackStatus) &{Scheduler}>

    access(all) init() {
        let storagePath = /storage/sharedScheduler
        let scheduler <- create UnsafeScheduler()
        self.account.storage.save(<-scheduler, to: storagePath)
        
        self.sharedScheduler = self.account.capabilities.storage.issue<auth(mayCancelCallback, mayReadCallbackStatus) &{Scheduler}>(storagePath)
    }

    access(all) fun schedule(
        callback: Capability<auth(mayExecuteCallback) &{CallbackHandler}>,
        data: AnyStruct?,
        timestamp: UFix64,
        priority: Priority,
        executionEffort: UInt64,
        fees: @FlowToken.Vault
    ): ScheduledCallback {
        return self.sharedScheduler.borrow()!.schedule(
            callback: callback, 
            data: data, 
            timestamp: timestamp, 
            priority: priority, 
            executionEffort: executionEffort, 
            fees: <-fees
        )
    }

    access(all) fun estimate(
        data: AnyStruct?,
        timestamp: UFix64,
        priority: Priority,
        executionEffort: UInt64
    ): EstimatedCallback? {
        return self.sharedScheduler.borrow()!
            .estimate(
                data: data, 
                timestamp: timestamp, 
                priority: priority, 
                executionEffort: executionEffort,
            )
    }

    access(all) fun getStatus(ID: UInt64): Status {
        return self.sharedScheduler.borrow()!.getStatus(ID: ID)
    }

    access(all) fun process() {
        self.sharedScheduler.borrow()!.process()
    }

    access(all) fun executeCallback(ID: UInt64) {
        self.sharedScheduler.borrow()!.executeCallback(ID: ID)
    }

    // Helper function to print the callback queue
    access(all) fun printCallbackQueue() {
        self.sharedScheduler.borrow()!.printCallbackQueue()
    }
}