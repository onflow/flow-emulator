import RandomBeaconHistory from 0xRANDOMBEACONHISTORYADDRESS

transaction {
    prepare(serviceAccount: auth(BorrowValue) &Account) {
        let randomBeaconHistoryHeartbeat = serviceAccount.storage
            .borrow<&RandomBeaconHistory.Heartbeat>(from: RandomBeaconHistory.HeartbeatStoragePath)
            ?? panic("Couldn't borrow RandomBeaconHistory.Heartbeat Resource")
        randomBeaconHistoryHeartbeat.heartbeat(randomSourceHistory: randomSourceHistory())
    }
}
