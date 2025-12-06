#!/bin/bash
# Test script for building Sliver C2 on EC2 via SSM

INSTANCE_ID="i-0080bcec99ef6fbf2"
REGION="us-west-2"

echo "Testing Sliver build on EC2 instance $INSTANCE_ID"

# Build warpgate binary on EC2
echo "Building warpgate binary..."
aws ssm send-command \
    --instance-ids "$INSTANCE_ID" \
    --region "$REGION" \
    --document-name "AWS-RunShellScript" \
    --parameters 'commands=["sudo -u ubuntu bash -c \"export PATH=/home/ubuntu/.asdf/shims:\$PATH && cd /home/ubuntu/warpgate && go build -o warpgate ./cmd/warpgate && ls -la warpgate\""]' \
    --output text \
    --query 'Command.CommandId'

echo "Waiting for build to complete..."
sleep 60

# Run full Sliver build with Ansible provisioners
echo "Running Sliver build with Ansible..."
COMMAND_ID=$(aws ssm send-command \
    --instance-ids "$INSTANCE_ID" \
    --region "$REGION" \
    --document-name "AWS-RunShellScript" \
    --parameters 'commands=["sudo -u ubuntu bash -c \"cd /home/ubuntu/warpgate && export PATH=/home/ubuntu/.asdf/shims:\$PATH && export ARSENAL_REPO_PATH=/home/ubuntu/ansible-collection-arsenal && ./warpgate build /home/ubuntu/warpgate-templates/templates/sliver/warpgate.yaml --target container --verbose 2>&1 | tee /tmp/warpgate-build.log\""]' \
    --output text \
    --query 'Command.CommandId')

echo "Build Command ID: $COMMAND_ID"
echo "Monitoring build..."

# Monitor build progress
for i in {1..120}; do
    sleep 10
    STATUS=$(aws ssm get-command-invocation \
        --instance-id "$INSTANCE_ID" \
        --region "$REGION" \
        --command-id "$COMMAND_ID" \
        --output text \
        --query 'Status' 2>/dev/null || echo "InProgress")

    echo "[Check $i] Status: $STATUS"

    if [ "$STATUS" = "Success" ] || [ "$STATUS" = "Failed" ]; then
        break
    fi
done

# Get final output
echo ""
echo "===== BUILD OUTPUT ====="
aws ssm get-command-invocation \
    --instance-id "$INSTANCE_ID" \
    --region "$REGION" \
    --command-id "$COMMAND_ID" \
    --output json | jq -r '.StandardOutputContent, .StandardErrorContent'

# Verify image
echo ""
echo "===== VERIFY IMAGE ====="
aws ssm send-command \
    --instance-ids "$INSTANCE_ID" \
    --region "$REGION" \
    --document-name "AWS-RunShellScript" \
    --parameters 'commands=["sudo podman images | grep sliver"]' \
    --output text \
    --query 'Command.CommandId'
