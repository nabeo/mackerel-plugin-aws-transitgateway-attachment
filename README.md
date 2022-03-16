# mackerel-plugin-aws-transitgateway-attachment

```
$ mackerel-plugin-aws-transitgateway-attachment -help
Usage of mackerel-plugin-aws-transitgateway-attachment:
  -access-key-id string
        AWS Access Key ID
  -metric-key-prefix string
        Metric Key Prefix
  -region string
        AWS Region
  -role-arn string
        IAM Role ARN for assume role
  -secret-key-id string
        AWS Secret Access Key ID
  -tgw string
        Resource ID of Transit Gateway
  -tgw-attch string
        Resource ID of Transit Gateway Attachement
nabeop@nabeop-d9ec2-01:~$
```

## use Assume Role

create IAM Role with the AWS Account that created Transit Gateway Attachment.

- no MFA
- allowed Policy
    - CloudWatchReadOnlyAccess

create IAM Policy with the AWS Account that runs mackerel-plugin-aws-transitgateway-attachment.

```json
{
    "Version": "2012-10-17",
    "Statement": {
        "Effect": "Allow",
        "Action": "sts:AssumeRole",
        "Resource": "arn:aws:iam::123456789012:role/YourIAMRoleName"
    }
}
```

attach IAM Policy to AWS Resouce that runs mackerel-plugin-aws-transitgateway-attachment.

## Synopsis

use assume role.
```shell
mackerel-plugin-aws-transitgateway-attachment -role-arn <IAM Role Arn> -region <region> \
                                               -tgw <Resource ID of Transit Gateway> \
                                               -tgw-attch <Resource ID of Transit Gateway Attachment>
```

use access key id and secret key.
```shell
mackerel-plugin-aws-transitgateway-attachment -region <region> \
                                               -tgw <Resource ID of Transit Gateway> \
                                               -tgw-attch <Resource ID of Transit Gateway Attachment> \
                                               [-access-key-id <AWS Access Key ID> -secret-key-id <WS Secret Access Key ID>] \
```
