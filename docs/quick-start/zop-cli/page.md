# Zop CLI

## Overview

Welcome to zop-cli – your streamlined solution for deploying **gofr** applications to the cloud with ease and efficiency. 
Zop-cli can automate the deployment process ensuring that your applications are up and running in the cloud with minimal effort.

With zop-cli, you can:
- Reduce deployment efforts across multiple cloud providers (AWS, Azure, GCP, etc.)
- Deploy GoFr applications in one command
- Focus on the core application logic and not worry about the huge devops overhead

### Key Features
- **Multi-Cloud Support**: Deploy to AWS, Azure, GCP, and more.
- **Easy Deployments**: Reduce deployment efforts and not going through huge CI/CD pipelines for smaller projects

## Get Started

Follow the instructions below to quickly set up and deploy your code in the cloud. Whether you’re new to cloud deployment or an experienced engineer, zop-cli will make your deployment process smoother and more reliable.

### Install CLI
Use the following command to get the cli installed on your machine
```bash
go install zop.dev@latest
```
OR build the application from binary
```bash
git clone https://github.com/kops-dev/kops-cli

go build -o zop.dev .
```

### Download Deployment key
1. Head over to zop.dev and navigate to the service that you are trying to deploy.
2. Click on the deploy key and hed over to the CLI tab to download the deployment key.
3. Place the deployment key in your local system and set the environment variable using the following command.
    ```bash
    export KOPS_DEPLOYMENT_KEY=<path_to_deployment_key>
    ```
   
### Deploy service to cloud
Now all that needs to be done is deploy the service to cloud using the following command
```bash
zop.dev deploy -name=<service_name> -tag=<image_tag>
```
> Note: Please make sure that the service name is the same as the service name is same as the service created on the zop.dev platform