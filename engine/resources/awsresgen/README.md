# awsresgen - AWS Cloud Control Resource Generator

This tool generates mgmt resource files from AWS CloudFormation resource type
schemas. Each generated resource uses the AWS Cloud Control API for unified CRUD
operations.

## How It Works

1. CloudFormation schemas (JSON) in `schemas/` describe AWS resource types
   including their properties, types, constraints, and supported operations.

2. The generator reads these schemas and produces typed Go resource files
   (`aws_<service>_gen.go`) in `engine/resources/`.

3. Each generated resource uses the shared Cloud Control client library
   (`engine/resources/aws/`) for all AWS API interactions.

4. Generated resources use mgmt's built-in polling mechanism for state
   monitoring (no push events from Cloud Control API).

## Usage

### Download schemas and generate resources

From the repository root:

```
make -C engine/resources/awsresgen download   # Fetch schemas from AWS
make -C engine/resources/awsresgen generate   # Generate Go resource files
```

The `download` target fetches CloudFormation JSON schemas from the public AWS
schema registry at:

```
https://schema.cloudformation.us-east-1.amazonaws.com/aws-<service>-<resource>.json
```

The list of resource types to download is defined in `RESOURCE_TYPES` in the
Makefile. Schemas are saved to `schemas/` with the naming convention
`AWS_<Service>_<Resource>.json`.

### Add a new resource type

1. Find the type name:

```
make list | grep -i ecs           # unauthenticated (downloads ~12MB zip)
make list-aws | grep -i ecs       # authenticated (requires AWS CLI + credentials)
```

2. Add the type name to `resources.txt`:

```
AWS::ECS::Cluster
```

3. Download and regenerate:

```
make download    # unauthenticated (public registry URLs)
make generate
```

Or with AWS credentials:

```
make download-auth    # authenticated (AWS CLI describe-type)
make generate
```

4. Build mgmt to verify:

```
cd ../../.. && make build
```

### Build without AWS support

To build mgmt without AWS resources (e.g., on systems without internet access
or to reduce binary size):

```
GOTAGS='noaws' make build
```

All generated files and the `aws/` client library use the `//go:build !noaws`
build tag.

## Using AWS Resources in MCL

AWS resources require polling since the Cloud Control API does not support
push-based notifications. Set the `Meta:poll` metaparam to a desired interval
in seconds.

### Example: Create an S3 Bucket

```mcl
aws:s3:bucket "my-bucket" {
    bucketname => "my-unique-bucket-name-12345",
    region => "us-east-1",

    Meta:poll => 30,
}
```

### Example: Create an EC2 VPC

```mcl
aws:ec2:vpc "my-vpc" {
    cidrblock => "10.0.0.0/16",
    region => "us-east-1",

    Meta:poll => 30,
}
```

### Example: Delete a resource

```mcl
aws:s3:bucket "old-bucket" {
    state => "absent",
    bucketname => "bucket-to-delete",
    region => "us-east-1",

    Meta:poll => 30,
}
```

### Authentication

Resources use the standard AWS credential chain:

1. Environment variables (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`)
2. Shared credentials file (`~/.aws/credentials`)
3. IAM instance role (when running on EC2)

The region can be set per-resource via the `region` field, or globally via the
`AWS_REGION` or `AWS_DEFAULT_REGION` environment variables.

## Generated Resource Types

| Kind | CloudFormation Type |
|------|---------------------|
| `aws:s3:bucket` | `AWS::S3::Bucket` |
| `aws:ec2:instance` | `AWS::EC2::Instance` |
| `aws:ec2:vpc` | `AWS::EC2::VPC` |
| `aws:ec2:subnet` | `AWS::EC2::Subnet` |
| `aws:ec2:securitygroup` | `AWS::EC2::SecurityGroup` |
| `aws:iam:role` | `AWS::IAM::Role` |
| `aws:rds:dbinstance` | `AWS::RDS::DBInstance` |
| `aws:lambda:function` | `AWS::Lambda::Function` |
| `aws:dynamodb:table` | `AWS::DynamoDB::Table` |
| `aws:sns:topic` | `AWS::SNS::Topic` |
| `aws:sqs:queue` | `AWS::SQS::Queue` |
| `aws:elasticloadbalancingv2:loadbalancer` | `AWS::ElasticLoadBalancingV2::LoadBalancer` |
| `aws:route53:hostedzone` | `AWS::Route53::HostedZone` |

## Makefile Targets

| Target | Description |
|--------|-------------|
| Target | Auth | Description |
|--------|------|-------------|
| `make list` | No | List all ~1550 available types (downloads zip archive) |
| `make download` | No | Fetch schemas for types in `resources.txt` |
| `make download-all` | No | Fetch all ~1550 schemas (zip archive) |
| `make list-auth` | Yes | List all available types via AWS CLI |
| `make download-auth` | Yes | Fetch schemas for types in `resources.txt` via AWS CLI |
| `make download-all-auth` | Yes | Fetch all schemas via AWS CLI (slow, ~1550 API calls) |
| `make generate` | - | Generate Go resource files from downloaded schemas |
| `make clean` | - | Remove generated files and downloaded schemas |

The `-auth` targets require the `aws` CLI and valid AWS credentials. Set
`AWS_REGION` if needed (defaults to `us-east-1`).

### Listing and filtering

```
make list | grep -i ec2           # Unauthenticated (downloads ~12MB zip)
make list-auth | grep -i ec2      # Authenticated (AWS CLI, faster)
```

Output uses the exact CloudFormation type name (e.g., `AWS::EC2::Instance`)
which can be pasted directly into `resources.txt`.

## Architecture

```
engine/resources/
    aws/                     # Shared Cloud Control client library
        client.go            # AWS session + CRUD wrapper
        diff.go              # JSON Patch (RFC 6902) computation
        schema.go            # CFN schema parser
        types.go             # Type mapping utilities
    awsresgen/               # This code generator
        main.go              # Entry point
        generator.go         # Schema -> Go struct generation
        Makefile             # Build targets
        resources.txt        # Whitelist of types to download/generate
        templates/
            aws_resource.go.tpl  # Go template for resource files
        schemas/             # Downloaded CFN JSON schemas (not committed)
    aws_*_gen.go             # Generated resource files (not committed)
```

## Schema Sources

CloudFormation schemas are publicly available from the AWS schema registry:

- **Individual schemas**: `https://schema.cloudformation.us-east-1.amazonaws.com/aws-<service>-<resource>.json`
- **Full archive**: `https://schema.cloudformation.us-east-1.amazonaws.com/CloudformationSchema.zip` (~1550 schemas)
- **AWS CLI**: `aws cloudformation describe-type --type RESOURCE --type-name AWS::S3::Bucket`

Schemas follow [JSON Schema draft-07](https://json-schema.org/draft-07/json-schema-release-notes.html)
with CloudFormation-specific extensions for read-only properties, create-only
properties, primary identifiers, and handler permissions.
