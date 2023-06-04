# OpenSearch Kubernetes Operator Upgrade Checker

We're excited to introduce the Upgrade Checker, a new feature in our OpenSearch Kubernetes (k8s) Operator. The Upgrade Checker not only ensures your Operator is up-to-date, but also allows us to gather valuable usage data to improve OpenSearch cluster services worldwide.

## What is the Upgrade Checker?

The Upgrade Checker is an integrated component of the OpenSearch k8s Operator. Each day at midnight, the Operator communicates with our Upgrade Checker server to verify if it's running the latest version. If an update is needed, a log message will be printed to remind you.

## Why Do We Collect Information?

The Upgrade Checker does more than keep your software current. When the Operator interacts with the Upgrade Checker, it sends along a unique identifier (UID), the number of OpenSearch clusters it manages, and their version numbers.

This data gives us crucial insights into how our software is being used and how OpenSearch clusters are being deployed around the world. With this information, we can identify popular versions, common cluster sizes, and general adoption trends.

## Data Transmission Details

The Upgrade Checker generates and sends a JSON file containing the following information to our server:

```json
{
	"properties": {
		"clusterCount": 3,
		"operatorVersion": "2.2.1",
		"osClustersVersions": [
			"2.4.0",
			"2.5.0",
			"2.5.0"
		],
		"uid": "8HcTRnKaGe"
	}
}
```

Each field represents:

- **clusterCount**: The number of OpenSearch clusters managed by the Operator.
- **operatorVersion**: The current version of the Operator.
- **osClustersVersions**: The versions of the managed OpenSearch clusters.
- **uid**: A unique identifier (UID) generated upon each new installation or when upgrading to a version of the Operator that includes the Upgrade Checker.

## Usage of Data and Privacy

The information is forwarded to our Segment instance, where we use it to generate visualizations, graphs, and dashboards, offering insights into Operator usage. This data aids our decision-making regarding future improvements and features.

We designed the Upgrade Checker with strict adherence to privacy principles. The information transmitted does not include any personal or sensitive data, just details about the OpenSearch Operator version and the clusters it manages.

## Supporting This Initiative

By allowing the Upgrade Checker to operate, you contribute to our understanding of OpenSearch usage patterns globally. This collective effort benefits all OpenSearch users, making the experience better for everyone.

## Disabling the Upgrade Checker

While we believe in the value of this initiative, we respect your choice if you wish not to participate. The Upgrade Checker is enabled by default but can be turned off at any time. To disable it, adjust the 'UpgradeChecker' value to 'false' in your helm chart. If you need further assistance, please refer to our comprehensive documentation or contact our support team.

Thank you for your continued support and usage of the OpenSearch Kubernetes Operator. Your contributions are helping us enhance data search and analysis capabilities worldwide.
