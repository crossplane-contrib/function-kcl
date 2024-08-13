"""
Copy from https://github.com/vfarcic/crossplane-kubernetes/blob/main/package/compositions.yaml
"""
crossplane = "1.14.5"
argocd = "3.35.4"
dapr = "1.12.4"
traefik = "26.0.0"
dynatraceOperator = "0.15.0"
dynatraceDashboard = "0.2.2"
externalSecrets = "0.9.11"
cilium = "1.14.2"
openFunctionUrl = "https://openfunction.github.io/charts/openfunction-v1.2.0-v0.7.0.tgz"
oxr = option("params").oxr
ocds = option("params").ocds
_metadata = lambda suffix: str -> any {
    {
        name = oxr.spec.id
        annotations = {
            "crossplane.io/external-name" = oxr.spec.id
            "krm.kcl.dev/composition-resource-name" = oxr.spec.id + "-" + suffix
        }
    }
}
schema usage:
    _nameSuffix: str
    _kind: str = "Object"
    apiVersion = "apiextensions.crossplane.io/v1alpha1"
    kind = "Usage"
    metadata = {
        name = oxr.spec.id + "-" + _nameSuffix + "-usage"
        annotations = {
            "crossplane.io/external-name" = oxr.spec.id + "-" + _nameSuffix + "-usage"
            "krm.kcl.dev/composition-resource-name" = oxr.spec.id + "-" + _nameSuffix + "-usage"
        }
    }
    spec = {
        of = {
            apiVersion = "container.gcp.upbound.io/v1beta2"
            kind = "Cluster"
            resourceRef.name = oxr.spec.id
        }
        by = {
            if _kind == "Object":
                apiVersion = "kubernetes.crossplane.io/v1alpha2"
            elif _kind == "Release":
                apiVersion = "helm.crossplane.io/v1beta1"
            kind = _kind
            resourceRef.name = oxr.spec.id + "-" + _nameSuffix
        }
    }
schema chart:
    _name: str
    _chartName?: str
    _chartRepository?: str
    _chartVersion?: str
    _chartUrl?: str
    _namespace: str
    _values?: any
    _providerConfigRefName?: str
    apiVersion = "helm.crossplane.io/v1beta1"
    kind = "Release"
    metadata = {
        name = oxr.spec.id + "-app-" + _name
        annotations = {
            "crossplane.io/external-name" = _name
            "krm.kcl.dev/composition-resource-name" = oxr.spec.id + "-app-" + _name
        }
    }
    spec = {
        forProvider = {
            chart = {
                if _chartName:
                    name = _chartName
                else:
                    name = _name
                if _chartRepository:
                    repository = _chartRepository
                if _chartVersion:
                    version = _chartVersion
                if _chartUrl:
                    url = _chartUrl
            }
            if _values:
                values = _values
            namespace = _namespace
        }
        rollbackLimit = 3
        if _providerConfigRefName:
            providerConfigRef.name = _providerConfigRefName
        else:
            providerConfigRef.name = oxr.spec.id
    }
schema object:
    _name: str
    _externalName?: str
    _manifest: any
    _references?: []
    apiVersion = "kubernetes.crossplane.io/v1alpha2"
    kind = "Object"
    metadata = {
        name = oxr.spec.id + "-app-" + _name
        annotations = {
            if _externalName:
                "crossplane.io/external-name" = _externalName
            else:
                "crossplane.io/external-name" = _name
            "krm.kcl.dev/composition-resource-name" = oxr.spec.id + "-app-" + _name
        }
    }
    spec = {
        if _references:
            references = _references
        forProvider.manifest = _manifest
        providerConfigRef.name = oxr.spec.id
    }
_items = [
    {
        apiVersion = "helm.crossplane.io/v1beta1"
        kind = "ProviderConfig"
        metadata = {
            name = oxr.spec.id
            annotations = {
                "krm.kcl.dev/ready": "True"
                "crossplane.io/external-name" = oxr.spec.id
                "krm.kcl.dev/composition-resource-name" = oxr.spec.id + "-config-helm"
            }
        }
        spec = {
            credentials = {
                secretRef = {
                    namespace = oxr.spec.claimRef.namespace
                    key = "kubeconfig"
                    name = oxr.spec.id + "-cluster"
                }
                source = "Secret"
            }
            if oxr.spec.compositionSelector.matchLabels.provider == "google":
                identity = {
                    type = "GoogleApplicationCredentials"
                    source = "Secret"
                    secretRef = {
                        name = "gcp-creds"
                        namespace = "crossplane-system"
                        key = "creds"
                    }
                }
        }
    }
    {
        apiVersion = "helm.crossplane.io/v1beta1"
        kind = "ProviderConfig"
        metadata = {
            name = oxr.spec.id + "-local"
            annotations = {
                "krm.kcl.dev/ready": "True"
                "crossplane.io/external-name" = oxr.spec.id
                "krm.kcl.dev/composition-resource-name" = oxr.spec.id + "-config-helm-local"
            }
        }
        spec.credentials.source = "InjectedIdentity"
    }
    if oxr.spec.compositionSelector.matchLabels.provider != "aws":
        {
            apiVersion = "helm.crossplane.io/v1beta1"
            kind = "Release"
            metadata = {
                name = oxr.spec.id + "-cilium"
                annotations = {
                    "crossplane.io/external-name" = "cilium"
                    "krm.kcl.dev/composition-resource-name" = oxr.spec.id + "-app-cilium"
                }
            }
            spec = {
                forProvider = {
                    chart = {
                        name = "cilium"
                        repository = "https://helm.cilium.io"
                        version = cilium
                    }
                    set = [
                        if oxr.spec.compositionSelector.matchLabels.provider == "google":
                            {name = "nodeinit.enabled", value = "true"}
                            {name = "nodeinit.reconfigureKubelet", value = "true"}
                            {name = "nodeinit.removeCbrBridge", value = "true"}
                            {name = "cni.binPath", value = "/home/kubernetes/bin"}
                            {name = "gke.enabled", value = "true"}
                            {name = "ipam.mode", value = "kubernetes"}
                            {
                                name = "ipv4NativeRoutingCIDR"
                                if oxr.spec.id + "-nodepool" in ocds:
                                    value = oxr.status.field1
                            },
                            {name = "authentication.mutual.spire.enabled", value = "true"}
                            {name = "authentication.mutual.spire.install.enabled", value = "true"}
                        if oxr.spec.compositionSelector.matchLabels.provider == "azure":
                            {name = "aksbyocni.enabled", value = "true"}
                            {name = "nodeinit.enabled", value = "true"}
                            {name = "authentication.mutual.spire.enabled", value = "true"}
                            {name = "authentication.mutual.spire.install.enabled", value = "true"}
                    ]
                    namespace = "kube-system"
                }
                rollbackLimit = 3
                providerConfigRef.name = oxr.spec.id
            }
        }
        if oxr.spec.parameters?.usage?.enabled:
            usage { _nameSuffix = "cilium", _kind = "Release" }
    {
        apiVersion = "kubernetes.crossplane.io/v1alpha1"
        kind = "ProviderConfig"
        metadata = {
            name = oxr.spec.id
            annotations = {
                "krm.kcl.dev/ready": "True"
                "crossplane.io/external-name" = oxr.spec.id
                "krm.kcl.dev/composition-resource-name" = oxr.spec.id + "-config-kubernetes"
            }
        }
        spec = {
            credentials = {
                secretRef = {
                    key = "kubeconfig"
                    name = oxr.spec.id + "-cluster"
                    namespace = oxr.spec.claimRef.namespace
                }
                source: "Secret"
            }
            if oxr.spec.compositionSelector.matchLabels.provider == "google":
                identity = {
                    type = "GoogleApplicationCredentials"
                    source = "Secret"
                    secretRef = {
                        name = "gcp-creds"
                        namespace = "crossplane-system"
                        key = "creds"
                    }
                }
        }
    }
    if oxr.spec.parameters?.apps?.crossplane?.enabled:
        chart {
            _name = "crossplane"
            _chartRepository = "https://charts.crossplane.io/stable"
            _chartVersion = crossplane
            _namespace = "crossplane-system"
        }
        if oxr.spec.parameters?.usage?.enabled:
            usage { _nameSuffix = "crossplane", _kind = "Release" }
    if oxr.spec.parameters?.apps?.argocd?.enabled:
        chart {
            _name = "argo-cd"
            _chartRepository = "https://argoproj.github.io/argo-helm"
            _chartVersion = argocd
            _namespace = "argocd"
            _values = {
                global.domain = oxr.spec.parameters.apps.argocd.host
                configs = {
                    secret = {
                        argocdServerAdminPassword = "$2a$10$m3eTlEdRen0nS86c5Zph5u/bDFQMcWZYdG3NVdiyaACCqoxLJaz16"
                        argocdServerAdminPasswordMtime = "2021-11-08T15:04:05Z"
                    }
                    cm = {
                        "application.resourceTrackingMethod" = "annotation"
                        "timeout.reconciliation" = "60s"
                    }
                    params = {
                        "server.insecure" = True
                    }
                }
                server = {
                    if oxr.spec.parameters?.apps?.traefik?.enabled:
                        ingress = {
                            enabled = True
                            ingressClassName = "traefik"
                        }
                    extraArgs = ["--insecure"]
                }
            }
        }
        object {
            _name = "argo-cd-app"
            _manifest = {
                apiVersion = "argoproj.io/v1alpha1"
                kind = "Application"
                metadata = {
                    name = "apps"
                    namespace = "argocd"
                    finalizers = ["resources-finalizer.argocd.argoproj.io"]
                }
                spec = {
                    project = "default"
                    source = {
                        repoURL = oxr.spec.parameters.apps.argocd.repoURL
                        targetRevision = "HEAD"
                        path = oxr.spec.parameters.apps.argocd.sourcePath
                    }
                    destination = {
                        server = "https://kubernetes.default.svc"
                        namespace = oxr.spec.parameters.apps.argocd.destinationNamespace
                    }
                    syncPolicy.automated = {
                        selfHeal = True
                        prune = True
                        allowEmpty = True
                    }
                }
            }
        }
        if oxr.spec.parameters?.usage?.enabled:
            usage { _nameSuffix = "argo-cd", _kind = "Release" }
            usage { _nameSuffix = "argo-cd-app" }
    if oxr.spec.parameters?.apps?.openfunction?.enabled:
        chart {
            _name = "openfunction"
            _chartUrl = openFunctionUrl
            _namespace = "openfunction"
            _values = {
                revisionController.enable = True
            }
        }
        if oxr.spec.parameters?.usage?.enabled:
            usage { _nameSuffix = "openfunction", _kind = "Release" }
    if oxr.spec.parameters?.apps?.dapr?.enabled:
        chart {
            _name = "dapr"
            _chartRepository = "https://dapr.github.io/helm-charts/"
            _chartVersion = dapr
            _namespace = "dapr-system"
        }
        if oxr.spec.parameters?.usage?.enabled:
            usage { _nameSuffix = "dapr", _kind = "Release" }
    if oxr.spec.parameters?.apps?.traefik?.enabled:
        chart {
            _name = "traefik"
            _chartRepository = "https://helm.traefik.io/traefik"
            _chartVersion = traefik
            _namespace = "traefik"
        }
        if oxr.spec.parameters?.usage?.enabled:
            usage { _nameSuffix = "traefik", _kind = "Release" }
    if oxr.spec.parameters?.apps?.dynatrace?.enabled:
        chart {
            _name = "dynatrace-operator"
            _chartRepository = "https://raw.githubusercontent.com/Dynatrace/dynatrace-operator/main/config/helm/repos/stable"
            _chartVersion = dynatraceOperator
            _namespace = "dynatrace"
            _values = {
                installCRD = True
                csidriver.enabled = True
            }
        }
        object {
            _name = "dynakube"
            _manifest = {
                apiVersion = "dynatrace.com/v1beta1"
                kind = "DynaKube"
                metadata = {
                    name = oxr.spec.id
                    namespace = "dynatrace"
                    annotations = {
                        "feature.dynatrace.com/k8s-app-enabled" = "true"
                    }
                }
                spec = {
                    apiUrl = oxr.spec.parameters.apps.dynatrace.apiUrl
                    oneAgent.cloudNativeFullStack.image = ""
                    activeGate = {
                        capabilities = [
                            "kubernetes-monitoring"
                            "routing"
                            "metrics-ingest"
                            "dynatrace-api"
                        ]
                        image = ""
                        resources = {
                            requests = {
                                cpu = "500m"
                                memory = "512Mi"
                            }
                            limits = {
                                cpu = "1000m"
                                memory = "1.5Gi"
                            }
                        }
                    }
                }
            }
        }
        chart {
            _name = "dynatrace-dashboard"
            _chartName = "kubernetes-cluster"
            _chartRepository = "https://katharinasick.github.io/crossplane-observability-demo-dynatrace"
            _chartVersion = dynatraceDashboard
            _namespace = "dynatrace"
            _values = {
                oauthCredentialsSecretName = oxr.spec.parameters.apps.dynatrace.oathCredentialsSecretName
                cluster = oxr.spec.id
                dashboards = {
                    clusterOverview.enabled = True
                    crossplaneMetrics.enabled = False
                }
            }
            _providerConfigRefName = oxr.spec.id + "-local"
        }
        if oxr.spec.parameters?.usage?.enabled:
            usage { _nameSuffix = "dynatrace-operator", _kind = "Release" }
            usage { _nameSuffix = "dynakube" }
            usage { _nameSuffix = "dynatrace-dashboard", _kind = "Release" }
    if oxr.spec.parameters?.apps?.externalSecrets?.enabled:
        chart {
            _name = "external-secrets"
            _chartRepository = "https://charts.external-secrets.io"
            _chartVersion = externalSecrets
            _namespace = "external-secrets"
            _values = {
                installCRDs = True
            }
        }
        if oxr.spec.parameters?.usage?.enabled:
            usage { _nameSuffix = "external-secrets", _kind = "Release" }
    if oxr.spec.parameters?.apps?.externalSecrets?.enabled and oxr.spec.parameters.apps?.externalSecrets?.store:
        object {
            _name = "secret-store"
            _externalName = oxr.spec.compositionSelector.matchLabels.provider
            _manifest = {
                apiVersion = "external-secrets.io/v1beta1"
                kind = "ClusterSecretStore"
                metadata.name = oxr.spec.compositionSelector.matchLabels.provider
                if oxr.spec.compositionSelector.matchLabels.provider == "google":
                    spec.provider.gcpsm.auth.secretRef.secretAccessKeySecretRef = {
                        name: "gcp-creds"
                        key: oxr.spec.parameters.apps.externalSecrets.googleCredentialsKey
                        namespace: oxr.spec.parameters.creds.namespace
                    }
                elif oxr.spec.compositionSelector.matchLabels.provider == "azure":
                    spec.provider.azurekv = {
                        authType = "ManagedIdentity"
                        vaultUrl = oxr.spec.parameters.apps.externalSecrets.azureVaultUrl
                    }
                elif oxr.spec.compositionSelector.matchLabels.provider == "aws":
                    spec.provider.aws = {
                        service = "SecretsManager"
                        region = "us-east-1"
                        auth.secretRef = {
                            accessKeyIDSecretRef = {
                                name = oxr.spec.parameters.creds.name
                                key = oxr.spec.parameters.apps.externalSecrets.awsAccessKeyIDKey
                                namespace = oxr.spec.parameters.creds.namespace
                            }
                            secretAccessKeySecretRef = {
                                name = oxr.spec.parameters.creds.name
                                key = oxr.spec.parameters.apps.externalSecrets.awsSecretAccessKeyKey
                                namespace = oxr.spec.parameters.creds.namespace
                            }
                        }
                    }
            }
            if oxr.spec.compositionSelector.matchLabels.provider == "google":
                _references = [{
                    patchesFrom = {
                        apiVersion = "gcp.upbound.io/v1beta1"
                        kind = "ProviderConfig"
                        name = "default"
                        fieldPath = "spec.projectID"
                    }
                    toFieldPath = "spec.provider.gcpsm.projectID"
                }]
        }
        if oxr.spec.parameters?.usage?.enabled:
            usage { _nameSuffix = "secret-store" }
]
if oxr.spec.parameters?.apps?.externalSecrets?.secrets:
    _items += [{
        apiVersion = "kubernetes.crossplane.io/v1alpha2"
        kind = "Object"
        metadata = {
            name = oxr.spec.id + "-secret-" + _secret.toSecret
            annotations = {
                "crossplane.io/external-name" = _secret.toSecret
                "krm.kcl.dev/composition-resource-name" = oxr.spec.id + "-secret-" + _secret.toSecret
            }
        }
        spec = {
            forProvider.manifest = {
                apiVersion = "external-secrets.io/v1beta1"
                kind = "ExternalSecret"
                metadata = {
                    name: _secret.toSecret
                    namespace: _secret.toNamespace
                }
                spec = {
                    refreshInterval = "1h"
                    secretStoreRef = {
                        kind: "ClusterSecretStore"
                        name: oxr.spec.compositionSelector.matchLabels.provider
                    }
                    target = {
                        name = _secret.toSecret
                        creationPolicy = "Owner"
                        template.type = _secret.type
                    }
                    dataFrom = [{ extract.key = _secret.fromSecret }]
                }
            }
            providerConfigRef.name = oxr.spec.id
        }
    } for _secret in oxr.spec.parameters.apps.externalSecrets.secrets ]
    if oxr.spec.parameters?.usage?.enabled:
        _items += [ usage {
            _nameSuffix = "secret-" + _secret.toSecret
        } for _secret in oxr.spec.parameters.apps.externalSecrets.secrets ]
if oxr.spec.parameters?.namespaces:
    _items += [{
        apiVersion = "kubernetes.crossplane.io/v1alpha2"
        kind = "Object"
        metadata = {
            name = oxr.spec.id + "-ns-" + _namespace
            annotations = {
                "crossplane.io/external-name" = _namespace
                "krm.kcl.dev/composition-resource-name" = oxr.spec.id + "-ns-" + _namespace
            }
        }
        spec = {
            forProvider.manifest = {
                apiVersion = "v1"
                kind = "Namespace"
                metadata.name = _namespace
            }
            deletionPolicy = "Orphan"
            providerConfigRef.name = oxr.spec.id
        }
    } for _namespace in oxr.spec.parameters.namespaces ]
if oxr.spec.parameters?.creds:
    _items += [
        {
            apiVersion = "kubernetes.crossplane.io/v1alpha2"
            kind = "Object"
            metadata = {
                name = oxr.spec.id + "-creds"
                annotations = {
                    "crossplane.io/external-name" = oxr.spec.parameters.creds.name
                    "krm.kcl.dev/composition-resource-name" = oxr.spec.id + "-creds"
                }
            }
            spec = {
                references = [{
                    patchesFrom = {
                        apiVersion = "v1"
                        kind = "Secret"
                        name: oxr.spec.parameters.creds.name
                        namespace: oxr.spec.parameters.creds.namespace
                        fieldPath: "data." + _credReference
                    }
                    toFieldPath: "data." + _credReference
                } for _credReference in oxr.spec.parameters.creds.keys]
                forProvider = {
                    manifest = {
                        apiVersion = "v1"
                        kind = "Secret"
                        metadata = {
                            name = oxr.spec.parameters.creds.name
                            namespace = oxr.spec.parameters.creds.namespace
                        }
                    }
                }
                providerConfigRef.name = oxr.spec.id
            }
        }
        if oxr.spec.parameters?.usage?.enabled:
            usage { _nameSuffix = "creds" }
    ]
items = _items
