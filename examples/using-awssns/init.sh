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

# Wait for 5 seconds after starting localstack
sleep 5

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
aws configure --profile test-profile <<EOF
dummy1
dummy1
eu-central-1
json
EOF

# Step 4: Create SNS topic
topic_arn=$(aws --endpoint-url=http://localhost:4566 sns create-topic --name order-creation-events --region eu-central-1 --profile test-profile | grep -o 'arn[^"]*')

# Step 5: Set environment variables
# shellcheck disable=SC2129
echo "APP_VERSION=v0" >> examples/using-awssns/configs/.local.env
echo "APP_NAME=aws-sns-example" >> examples/using-awssns/configs/.local.env
echo "HTTP_PORT=8080" >> examples/using-awssns/configs/.local.env
echo "SNS_ACCESS_KEY=dummy1" >> examples/using-awssns/configs/.local.env
echo "SNS_SECRET_ACCESS_KEY=dummy1" >> examples/using-awssns/configs/.local.env
echo "SNS_REGION=eu-central-1" >> examples/using-awssns/configs/.local.env
echo "SNS_PROTOCOL=http" >> examples/using-awssns/configs/.local.env
echo "SNS_ENDPOINT=http://localhost:4566/" >> examples/using-awssns/configs/.local.env
echo "SNS_TOPIC_ARN=$topic_arn" >> examples/using-awssns/configs/.local.env
echo "NOTIFIER_BACKEND=SNS" >> examples/using-awssns/configs/.local.env