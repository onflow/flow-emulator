import FlowToken from 0x0ae53cb6e3f42a79

// UnsafeCallbackScheduler is a simple implementation of the CallbackScheduler interface.
//
// WARNING:
// It is unsafe because it does not implement safe scheduling logic and it only simulates priority.
// It is used on emulator for testing purposes.
// The APIs simulate the behaviour you can expect from production implementation.
access(all) contract UnsafeCallbackScheduler {

    access(all) entitlement Callback

    access(all) struct interface CallbackHandler {
        access(Callback) fun executeCallback(data: AnyStruct?)
    }

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
        access(all) case Rejected
    }

    access(all) struct ScheduledCallback {
        access(all) let ID: UInt64
        access(all) let timestamp: UFix64?
        access(all) let cancel: fun(): @FlowToken.Vault
        access(all) var status: Status

        access(contract) init(id: UInt64, timestamp: UFix64?, cancel: fun(): @FlowToken.Vault) {
            self.ID = id
            self.status = Status.Scheduled
            self.timestamp = timestamp
            self.cancel = cancel
        }
    }

    access(all) struct EstimatedCallback {
        access(all) var flowFee: UFix64
        access(all) var timestamp: UFix64

        init(flowFee: UFix64, timestamp: UFix64) {
            self.flowFee = flowFee
            self.timestamp = timestamp
        }
    }

    access(all) event CallbackScheduled(id: UInt64, timestamp: UFix64?, priority: UInt8, executionEffort: UInt64)
    access(all) event CallbackProcessed(id: UInt64, executionEffort: UInt64)
    access(all) event CallbackExecuted(id: UInt64)
    access(all) event CallbackCanceled(id: UInt64)

    access(all) resource CallbackData {
        access(all) let handler: Capability<auth(Callback) &{CallbackHandler}>
        access(all) let data: AnyStruct?
        access(all) let originalTimestamp: UFix64
        access(all) let priority: Priority
        access(all) let executionEffort: UInt64
        access(all) let fees: @FlowToken.Vault
        access(all) var status: Status
        access(all) let scheduledTimestamp: UFix64

        init(
            handler: Capability<auth(Callback) &{CallbackHandler}>,
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
    }


    access(self) var nextID: UInt64 // next callbackID to be used
    access(self) var callbacks: @{UInt64: CallbackData} // callbackID -> callbackData
    access(self) var timestampQueue: {UFix64: [UInt64]} // timestamp -> callbackIDs

    // Fee multipliers
    access(self) let HIGH_MULTIPLIER: UFix64
    access(self) let MEDIUM_MULTIPLIER: UFix64
    access(self) let LOW_MULTIPLIER: UFix64

    init() {
        self.nextID = 0
        self.callbacks <- {}
        self.timestampQueue = {}
        self.HIGH_MULTIPLIER = 10.0
        self.MEDIUM_MULTIPLIER = 5.0
        self.LOW_MULTIPLIER = 2.0
    }

    // Helper to calculate adjusted timestamp based on priority used for simulating priority
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

    // Helper to calculate fee
    access(self) fun calculateFee(executionEffort: UInt64, priority: Priority): UFix64 {
        let baseFee = self.executionEffortToFlow(executionEffort: executionEffort)
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

    // Helper to convert execution effort to Flow tokens
    // test implementation for simplicity
    access(all) fun executionEffortToFlow(executionEffort: UInt64): UFix64 {
        return UFix64(executionEffort) / 10000.0
    }

    access(all) fun schedule(
        callback: Capability<auth(Callback) &{CallbackHandler}>,
        data: AnyStruct?,
        timestamp: UFix64,
        priority: Priority,
        executionEffort: UInt64,
        fees: @FlowToken.Vault
    ): ScheduledCallback? {
        // Validate timestamp is in future
        if timestamp <= getCurrentBlock().timestamp {
            // todo we shouldn't destroy fees here, we should return the vault as part of
            // the scheduled callback with status rejeceted
            destroy fees
            return nil
        }

        // Calculate required fee
        let requiredFee = self.calculateFee(executionEffort: executionEffort, priority: priority)
        if fees.balance < requiredFee {
            destroy fees
            return nil
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

        // simulate timestamp unknown for low priority
        if priority == Priority.Low {
            scheduledTimestamp = nil
        }

        log("[scheduler.schedule] callback scheduled: ".concat(callbackID.toString()).concat(" timestamp: ").concat(scheduledTimestamp!.toString()).concat(" priority: ").concat(self.priorityToString(priority: priority)))

        emit CallbackScheduled(
            id: callbackID,
            timestamp: scheduledTimestamp,
            priority: priority.rawValue,
            executionEffort: executionEffort
        )

        let cancel = fun(): @FlowToken.Vault {
            let callback <- self.callbacks.remove(key: callbackID)
                ?? panic("Callback not found ".concat(callbackID.toString()))

            // Remove from timestamp queue
            self.timestampQueue.remove(key: callback.scheduledTimestamp)

            // Return half the fees
            let halfAmount = callback.fees.balance / 2.0
            let refundFees <- callback.fees.withdraw(amount: halfAmount) as! @FlowToken.Vault

            destroy callback

            log("[scheduler.cancel] callback canceled: ".concat(callbackID.toString()))

            emit CallbackCanceled(id: callbackID)
            return <- refundFees
        }

        return ScheduledCallback(id: callbackID, timestamp: scheduledTimestamp, cancel: cancel)
    }

    access(all) fun estimate(
        data: AnyStruct?,
        timestamp: UFix64,
        priority: Priority,
        executionEffort: UInt64
    ): EstimatedCallback? {
        // Reject if low priority
        if priority == Priority.Low {
            return nil
        }

        // Validate timestamp
        if timestamp <= getCurrentBlock().timestamp {
            return nil
        }

        let fee = self.calculateFee(executionEffort: executionEffort, priority: priority)
        let scheduledTimestamp = self.calculateScheduledTimestamp(timestamp: timestamp, priority: priority)

        return EstimatedCallback(flowFee: fee, timestamp: scheduledTimestamp)
    }

    access(all) fun process() {
        log("[scheduler.process] processing callbacks")

        let currentTimestamp = getCurrentBlock().timestamp

        // Find all timestamps that should be processed
        let timestampShouldBeProcessed = view fun (element: UFix64): Bool {
            return element <= currentTimestamp
        }
        let eligibleTimestamps = self.timestampQueue.keys.filter(timestampShouldBeProcessed)

        for timestamp in eligibleTimestamps {
            if let callbackIDs = self.timestampQueue[timestamp] {
                for callbackID in callbackIDs {
                    let callback = &self.callbacks[callbackID] as &CallbackData?
                    if callback != nil && callback!.status == Status.Scheduled {
                        log("  | callback processed: ".concat(callbackID.toString()))
                        callback!.setStatus(newStatus: Status.Processed)
                        emit CallbackProcessed(id: callbackID, executionEffort: callback!.executionEffort)
                    } else if callback != nil {
                        log("Ingoring already processed callback ".concat(callbackID.toString()))
                    }
                }
            }
        }
    }

    access(all) fun executeCallback(ID: UInt64) {
        log("[scheduler.executeCallback] executing callback ".concat(ID.toString()))

        let callback = &self.callbacks[ID] as &CallbackData?
        if callback == nil {
            panic("Callback not found ".concat(ID.toString()))
        }

        if callback!.status != Status.Processed {
            panic("Trying to execute callback that is not processed ".concat(ID.toString()).concat(" status: ").concat(self.statusToString(status: callback!.status)))
        }

        let handlerRef = callback!.handler.borrow()
            ?? panic("Cannot borrow callback handler")
        handlerRef.executeCallback(data: callback!.data)

        callback!.setStatus(newStatus: Status.Executed)
        emit CallbackExecuted(id: ID)
    }

    // Helper function to print the callback queue
    access(all) fun printCallbackQueue() {
        for timestamp in self.timestampQueue.keys {
            log("[scheduler.queue] timestamp: ".concat(timestamp.toString()))

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

    access(all) fun statusToString(status: Status): String {
        switch status {
            case Status.Scheduled:
                return "scheduled"
            case Status.Processed:
                return "processed"
            case Status.Executed:
                return "executed"
            case Status.Canceled:
                return "canceled"
            case Status.Rejected:
                return "rejected"
            default:
                panic("Invalid status ".concat(status.rawValue.toString()))
        }
    }

    access(all) fun priorityToString(priority: Priority): String {
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