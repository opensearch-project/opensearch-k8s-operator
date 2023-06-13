# Autoscaling

## Content
- [Autoscaling](#autoscaling)
- [Goals](#goals)
- [Limitations](#limitations)
- [Overview](#overview)
- [Questions](#questions)

## Goals
1. Scale OpenSearch clusters managed by the operator up and down via monitoring metrics.
2. Support for making scaling decision from one-to-many metrics with aggregations.

## Limitations
There are two identified limitations at the moment:
1. Any node containing a **master** role will be excluded from autoscaling operations. This is a purposeful decision to avoid split-brain scenarios with the cluster.
2. There are few metrics currently that support aggregated values over time coming from the prometheus-exporter. This is important because to make scaling decisions, using latest data point values will lead to flapping conditions as well non-smooth scaling operations. A method will need to be devised to support metric aggregations, whether this should live as part of the metrics component or the autoscaling component is yet to be determined. For now I am planning on this existing as part of metrics.

## Overview

### Separate CRD for autoscaler configurations.
Currently configuration for the CRD is maintained by an OpensearchCluster definiton. Specifically configurations for enabling things like the smartscaler and autoscaling have already been defined under the confMgmt section of the CRD. This is fine for determining the state of operation for autoscaling, however it is not the best place for configuring the actual autoscaling logic to be done due to it more than likely cluttering up the CRD. This means a seperate CRD for autoscaling is preferred that can be solely used for defining autoscaling rules.

### CRD Design Considerations/Assumptions:
Using a separate CRD for autoscaling will keep the rules that are defined organized in their own location to allow ease of maintenance and preserve readability of the core CRD. Some consideration need to be made to connect the OpenSearchCluster CRD with the autoscaler CRD. 

1. ConfMgmt currently contains the boolean to enable/disable autoscaling; This should be fine to remain here, however an additional field to define the autoscaler rules that we want to target would be beneficial. This would allow multiple autoscaling rules to be defined in the autoscaler CRD, without neccesarily having them active. Without this we would also need a way to tie certain rules to certain clusters and it seems cleaner to control the rules being used by the cluster CRD versus the autoscaler CRD. I can however see potential benefits to both approaches. More discussion needed.
2. The autoscaler will requires the following to operate:
    
    1. One or more metrics to make a scaling decision.
    2. The operand with which to compare the metric value and threshold.
    3. A threshold value that if exceeded, will trigger the scaling action.
    4. The action that is desired, i.e. scale up or down one node.
    5. The node type to scale. 

There are going to be cases where we want to scale when multiple metrics exceed specific thresholds, however, not just a single metric each time. The aforementioned four items comprise what I consider to be a rule "block", and the autoscaler should support groupings of these blocks if desired to allow scaling decisions to be made of more than one metric. **Note:** The scaling action and the node type are only required once per grouping of rule blocks.

3. I am also making a design assumption that rule blocks existing in one rule grouping will require all of their thesholds to be exceeded to perform a scaling operation. This is effectively an AND operation, where all operands must conclude a scaling action is warranted. 



<details>
  <summary>Dropdown for Example CRD</summary>
  
  ### Autoscaler CRD Example

  In this YAML definition, the autoscaler CRD has a spec property that includes an action, node-role, and rules property. The rules property is an array of objects, each containing a rule property which is itself an array of objects. Each object in the rule array contains a metric, operand, and threshold property.

  ```yaml
  apiVersion: apiextensions.k8s.io/v1
    kind: CustomResourceDefinition
    metadata:
    name: autoscaling.opensearchclusters.opensearch.opster.io
    spec:
    group: opensearch.opster.io
    names:
        kind: autoscaler
        plural: autoscalers
    scope: Namespaced
    versions:
        - name: v1
        served: true
        storage: true
        schema:
            openAPIV3Schema:
                type: object
            properties:
                apiVersion:
                    type: string
                kind:
                    type: string
                metadata:
                    type: object
                spec:
                    type: object
                properties:
                    action:
                        type: string
                    node-role:
                        type: string
                    rules:
                        type: array
                    items:
                        type: object
                        properties:
                            rule:
                                type: array
                            items:
                                type: object
                                properties:
                                    metric:
                                    type: string
                                    operand:
                                    type: string
                                    threshold:
                                    type: float
                                
  ```
</details>
<br>
<br>

### Operator Considerations
Once the ruleset is established for scaling operations, the logic needs to be put in place for checking and executing the rules. I plan on leveraging the existing nodePool reconciliation loop to determine if autoscaling is enabled,  and if so perform the relevant check to determine scaling. The autoscaler CRD will provide specifications for scaling, however the metrics to compare will need to be fetched. I am assuming this will be done via curling the specified endpoint however there may be a better way to do this that I am unaware of. Regardless of methodology in fetching, this should be a simple check to see if autoscaling is enabled for the cluster, if enabled, loop through the rules for the cluster and determine if scaling is needed or not, and end with scaling up or down one node.

<br>
<br>

### Questions:
1. Metrics coming from the prometheus-exporter are float64, but these values are not recommended for CRD generation. Do we want to AllowDangerousTypes for this?
2. Metric aggregation needs to be supported to avoid scaling during minor/short increases. Where and how these aggregations should happen are up for debate. I am not fully versed in how hard or easy it ould be to add something like this to existing metrics, but it seems like it should happen as part of metrics? 
3. Does it make more sense to have the OpenSearchCluster CRD specify the rules(name of rule grouping) that it should use or should the autoscaler CRD specifiy the OpenSearchCluster that the rules are being used for? Or is there another option that makes better sense. Originally I was leaning towards the former option, however if it is done that way then it becomes awkward to define multiple groupings of scaling rules for the same cluster. If we leave the target cluster to defined by the rules then there is no ambiguity about what is happening, but it requires that extra step in configuration.
4. If performing autoscaling logic as part of the nodePool reconciliation loop, what is the best way to get metrics from the preometheus-exporter? Is there an ability to request metrics by name, or is it a case of curling the already established endpoint?