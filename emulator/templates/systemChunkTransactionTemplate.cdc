import RandomBeaconHistory from 0xRANDOMBEACONHISTORYADDRESS

transaction {
    prepare(serviceAccount: AuthAccount) {
        let randomBeaconHistoryHeartbeat = serviceAccount.borrow<&RandomBeaconHistory.Heartbeat>(
            from: RandomBeaconHistory.HeartbeatStoragePath
        ) ?? panic("Couldn't borrow RandomBeaconHistory.Heartbeat Resource")
        randomBeaconHistoryHeartbeat.heartbeat(randomSourceHistory: randomSourceHistory())
    }
}
