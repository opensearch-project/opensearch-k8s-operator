# Autoscaling

## Content
- [Autoscaling](#autoscaling)
- [Goals](#goals)
- [Design](#design)
- [Getting Started](#gettingstarted)

## Goals
1. Scale OpenSearch clusters managed by the operator up and down via monitoring metrics.
2. Support for making scaling decision from one-to-many metrics with aggregations.

## Design
A separate CRD is used for defining autoscaling policies. Autoscaling CRDs are stateless as they are never updated by the operator and only read. Once an autoscaler is created, it can be referenced from either a cluster or nodepool level inside the OpensearchCluster configuration. When enabled, the autoscaler will query a prometheus backend containing the cluster metrics and make scaling determinations based on the user configuration. 

### Requirements
To support the second goal of being able to make scaling decisions with aggregations, there needs to be a record of cluster metrics over a time period. Since the monitoring component of the operator is already leveraging Prometheus, it made sense to utilize it as well. The autoscaler requires a Prometheus instance that is scraping the metrics of your cluster for the autoscaler to work. 

### Considerations
Some design considerations to make note of:
1. ScaleConf only contains maxReplicas but no minReplicas, this is because the number of replicas specified in the nodepool for the OpenSearch cluster is used for the minReplica value.
2. The operator field of an Item can be any supported Prometheus comparison binary operator.
```
== (equal)
!= (not-equal)
> (greater-than)
< (less-than)
>= (greater-or-equal)
<= (less-or-equal)
```
3. The interval field of a queryOption can be an integer follow by any valid Prometheus time duration.
```
ms - milliseconds
s - seconds
m - minutes
h - hours
d - days - assuming a day has always 24h
w - weeks - assuming a week has always 7d
y - years - assuming a year has always 365d
```
4. The function field of a queryOption can be any valid singular Prometheus function.

### Autoscaler Custom Resource Reference Guide

The Autoscaler CRD is defined by kind: `Autoscaler`, group: `opensearch.opster.io` and version `v1`.
| Name | Type | Description | Required |
|--------|--------|--------|--------|
| apiVersion | string | opensearch.opster.io/v1 | true |
| kind | string | Autoscaler | true |
| metadata | object | Refer to the Kubernetes API documentation for the fields of the `metadata` field. | true |
| spec | object | AutoscalerSpec defines the desired configuration of the autoscaler. | true |


### Autoscaler.spec
AutoscalerSpec defines the desired configuration of the autoscaler.
| Name | Type | Description | Required |
|--------|--------|--------|--------|
| rules | []Rule | The container for lists of type Rule, defining scaling logic. | true |


### Rule
Rule defines a single rule.
| Name | Type | Description | Required |
|--------|--------|--------|--------|
| items | []Item | A list of type Item, defining conditions for scaling. | true |
| nodeRole | string | The role of the Opensearch node type you would like to target for scaling. | true |
| behavior | Scale | The container for the scaling behavior of the ruleset. | true |

A rule may contain many items; by default all items expressions generated from the configuration must evaluate to true for a scaling action to take place.
A nodeRole is needed primarily in the case that the autoscalePolicy is defined at the cluster level so which nodes to scale is known.

### Item
Item defines a singular item in a rule.
| Name | Type | Description | Required |
|--------|--------|--------|--------|
| metric | string | A prometheus metric to target for performing conditional operations. | true |
| operator | string | The operator to use for comparing the prometheus query result and threshold. | true |
| threshold | string | The threshold value for taking scaling action. | true |
| queryOptions | QueryOptions | Optional additions to the prometheus query. | false |

The operator field of an Item can be any supported Prometheus comparison binary operator.
```
== (equal)
!= (not-equal)
> (greater-than)
< (less-than)
>= (greater-or-equal)
<= (less-or-equal)
```

### QueryOptions
QueryOptions defined additional query configurations.
| Name | Type | Description | Required |
|--------|--------|--------|--------|
| labelMatchers | []string | A prometheus supported label matcher to limit results. | false |
| interval | string | A prometheus supported interval of time over which to query. | false |
| function | string | A prometheus supported function wrapper. | false |
| aggregateEvaluation | bool | A flag to average your prometheus query results together. | false |

The interval field of a queryOption can be an integer follow by any valid Prometheus time duration.
```
ms - milliseconds
s - seconds
m - minutes
h - hours
d - days - assuming a day has always 24h
w - weeks - assuming a week has always 7d
y - years - assuming a year has always 365d
```

The aggregateEvaluation field is designed to average the results from multiple nodes for comparison. This is useful for when you want to scale based off an average of nodes metrics versus each individual node needing to be evaluated.

### Behavior
Behavior defines a scaling behavior for a rule.
| Name | Type | Description | Required |
|--------|--------|--------|--------|
| enable | bool | Flag to enable or disable the rule. | true |
| scaleUp | ScaleConf | Container for upscaling behavior. | false |
| scaleDown | ScaleConf | Container for downscaling behavior. | false |

You should never have both scaleUp and scaleDown defined for the same rule. Each rule should only ever have one or the other.

### ScaleConf
Scaling behavior for scaling up or down.
| Name | Type | Description | Required |
|--------|--------|--------|--------|
| maxReplicas | int32 | Maximum amount of replicas to scale up to. | false |

MaxReplicas is an optional field in the case of a rule that is scaling down, however if scaling up it is needed so there is an upper boundary. MinReplicas is absent because the nodepool.Replicas defined in the cluster spec performs this function. When scaling down the cluster will never scale below the number of replicas defined in the cluster.


In addition to the autoscaler CRD, changes to the existing OpensearchCluster CRD are included, specifically the generalConfig and nodePools.

### OpensearchCluster.General.Autoscaler
Addition of an `Autoscaler` section under generalConfig.
| Name | Type | Description | Required |
|--------|--------|--------|--------|
| enable | boolean | Enables or disables autoscaling functionality. | false |
| prometheusEndpoint | string | A prometheus endpoint to monitor. | false |
| scaleTimeout | int | The amount of time to wait before scaling since last scale or cluster creation in minutes. | false |
| clusterPolicy | string | The override to set a cluster specific autoscale policy. | false |

### OpensearchCluster.nodePools
Addition of `AutoScalePolicy` to nodePools.
| Name | Type | Description | Required |
|--------|--------|--------|--------|
| autoScalePolicy | string | The name of an autoscaler that the user has applied. | false |

Note that the clusterPolicy and autoScalePolicy are synonymous and users should choose one or the other based on their needs. 

## GettingStarted
1. Have a Prometheus instance where metrics from your cluster are being stored.
2. Create an autoscaling policy with the CRD that meets your scaling requirements.
3. Define the autoscaling policy in your OpensearchCluster and enable it.