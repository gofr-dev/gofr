set -e

# Function to install AWS CLI on Linux
install_aws_cli_linux() {
    echo "Installing AWS CLI on Linux..."
    if ! command -v curl &> /dev/null; then
        sudo apt-get update
        sudo apt-get install -y curl
    fi
    curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip"
    unzip awscliv2.zip
    sudo ./aws/install
    aws --version
}
# Function to install aws_cli
install_aws_cli_mac() {
    echo "Installing AWS CLI on macOS..."

    # Check if AWS CLI is already installed
    if command -v aws &> /dev/null; then
        echo "AWS CLI is already installed."
        aws --version
        return
    fi

    # Download the AWS CLI installation package
    echo "Downloading AWS CLI..."
    curl "https://awscli.amazonaws.com/AWSCLIV2.pkg" -o "AWSCLIV2.pkg"

    # Install AWS CLI
    echo "Installing AWS CLI..."
    sudo installer -pkg AWSCLIV2.pkg -target /

    # Verify installation
    aws --version

    # Clean up downloaded package
    echo "Cleaning up..."
    rm AWSCLIV2.pkg

    echo "AWS CLI installation completed."
}


# Step 1: Start localstack docker container
docker-compose -f examples/using-awssns/Docker-Compose.yml up -d

# Step 2: Check operating system and install AWS CLI if not installed
if [[ "$OSTYPE" == "linux-gnu"* ]]; then
    if ! command -v aws &> /dev/null; then
        install_aws_cli_linux
    fi
elif [[ "$OSTYPE" == "darwin"* ]]; then
    if ! command -v aws &> /dev/null; then
        install_aws_cli_mac
    fi
else
    echo "Unsupported operating system: $OSTYPE"
    exit 1
fi

# Step 3: Configure AWS CLI profile
aws configure --profile test-profile

aws configure set aws_access_key_id dummy1 --profile test-profile
aws configure set aws_secret_access_key dummy1 --profile test-profile
aws configure set region eu-central-1 --profile test-profile

# Step 4: Create SNS topic
topic_arn=$(aws --endpoint-url=http://localhost:4566 sns create-topic --name order-creation-events --region eu-central-1 --profile test-profile | grep -o 'arn[^"]*')

# Step 5: Set environment variables
echo "APP_VERSION=v0" > ./configs/.local.env
echo "APP_NAME=aws-sns-example" >> ./configs/.local.env
echo "HTTP_PORT=8080" >> ./configs/.local.env
echo "SNS_ACCESS_KEY=dummy1" >> ./configs/.local.env
echo "SNS_SECRET_ACCESS_KEY=dummy1" >> ./configs/.local.env
echo "SNS_REGION=eu-central-1" >> ./configs/.local.env
echo "SNS_PROTOCOL=http" >> ./configs/.local.env
echo "SNS_ENDPOINT=http://localhost:4566/" >> ./configs/.local.env
echo "SNS_TOPIC_ARN=$topic_arn" >> ./configs/.local.env
echo "NOTIFIER_BACKEND=SNS" >> ./configs/.local.env