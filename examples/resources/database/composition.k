items = [{
    apiVersion: "sql.gcp.upbound.io/v1beta1"
    kind: "DatabaseInstance"
    spec: {
        forProvider: {
            project: "test-project"
            settings: [{databaseFlags: [{
                name: "log_checkpoints"
                value: "on"
            }]}]
        }
    }
}]
