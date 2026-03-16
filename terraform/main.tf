terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region = var.aws_region
}

# ──────────────────────────────────────────
# ECR (already exists, just reference it)
# ──────────────────────────────────────────
data "aws_ecr_repository" "hw7" {
  name = "hw7"
}

# ──────────────────────────────────────────
# VPC
# ──────────────────────────────────────────
resource "aws_vpc" "main" {
  cidr_block           = "10.0.0.0/16"
  enable_dns_support   = true
  enable_dns_hostnames = true
  tags = { Name = "hw7-vpc" }
}

resource "aws_internet_gateway" "main" {
  vpc_id = aws_vpc.main.id
  tags   = { Name = "hw7-igw" }
}

# Public subnets (for ALB)
resource "aws_subnet" "public_1" {
  vpc_id                  = aws_vpc.main.id
  cidr_block              = "10.0.1.0/24"
  availability_zone       = "us-west-2a"
  map_public_ip_on_launch = true
  tags                    = { Name = "hw7-public-1" }
}

resource "aws_subnet" "public_2" {
  vpc_id                  = aws_vpc.main.id
  cidr_block              = "10.0.2.0/24"
  availability_zone       = "us-west-2b"
  map_public_ip_on_launch = true
  tags                    = { Name = "hw7-public-2" }
}

# Private subnets (for ECS)
resource "aws_subnet" "private_1" {
  vpc_id            = aws_vpc.main.id
  cidr_block        = "10.0.10.0/24"
  availability_zone = "us-west-2a"
  tags              = { Name = "hw7-private-1" }
}

resource "aws_subnet" "private_2" {
  vpc_id            = aws_vpc.main.id
  cidr_block        = "10.0.11.0/24"
  availability_zone = "us-west-2b"
  tags              = { Name = "hw7-private-2" }
}

# Route table for public subnets
resource "aws_route_table" "public" {
  vpc_id = aws_vpc.main.id
  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.main.id
  }
  tags = { Name = "hw7-public-rt" }
}

resource "aws_route_table_association" "public_1" {
  subnet_id      = aws_subnet.public_1.id
  route_table_id = aws_route_table.public.id
}

resource "aws_route_table_association" "public_2" {
  subnet_id      = aws_subnet.public_2.id
  route_table_id = aws_route_table.public.id
}

# ──────────────────────────────────────────
# Security Groups
# ──────────────────────────────────────────
resource "aws_security_group" "alb" {
  name   = "hw7-alb-sg"
  vpc_id = aws_vpc.main.id

  ingress {
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_security_group" "ecs" {
  name   = "hw7-ecs-sg"
  vpc_id = aws_vpc.main.id

  ingress {
    from_port       = 8080
    to_port         = 8080
    protocol        = "tcp"
    security_groups = [aws_security_group.alb.id]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

# ──────────────────────────────────────────
# SNS + SQS
# ──────────────────────────────────────────
resource "aws_sns_topic" "orders" {
  name = "order-processing-events"
}

resource "aws_sqs_queue" "orders" {
  name                       = "order-processing-queue"
  visibility_timeout_seconds = 30
  message_retention_seconds  = 345600 # 4 days
  receive_wait_time_seconds  = 20     # long polling
}

resource "aws_sqs_queue_policy" "orders" {
  queue_url = aws_sqs_queue.orders.url
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "sns.amazonaws.com" }
      Action    = "sqs:SendMessage"
      Resource  = aws_sqs_queue.orders.arn
      Condition = {
        ArnEquals = { "aws:SourceArn" = aws_sns_topic.orders.arn }
      }
    }]
  })
}

resource "aws_sns_topic_subscription" "orders" {
  topic_arn = aws_sns_topic.orders.arn
  protocol  = "sqs"
  endpoint  = aws_sqs_queue.orders.arn
}

# ──────────────────────────────────────────
# IAM - use existing LabRole (Learner Lab restriction)
# ──────────────────────────────────────────
data "aws_iam_role" "lab_role" {
  name = "LabRole"
}

# ──────────────────────────────────────────
# ALB
# ──────────────────────────────────────────
resource "aws_lb" "main" {
  name               = "hw7-alb"
  internal           = false
  load_balancer_type = "application"
  security_groups    = [aws_security_group.alb.id]
  subnets            = [aws_subnet.public_1.id, aws_subnet.public_2.id]
}

resource "aws_lb_target_group" "receiver" {
  name        = "hw7-receiver-tg"
  port        = 8080
  protocol    = "HTTP"
  vpc_id      = aws_vpc.main.id
  target_type = "ip"

  health_check {
    path                = "/health"
    interval            = 30
    healthy_threshold   = 2
    unhealthy_threshold = 3
  }
}

resource "aws_lb_listener" "main" {
  load_balancer_arn = aws_lb.main.arn
  port              = 80
  protocol          = "HTTP"

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.receiver.arn
  }
}

# ──────────────────────────────────────────
# ECS Cluster
# ──────────────────────────────────────────
resource "aws_ecs_cluster" "main" {
  name = "hw7-cluster"
}

# ──────────────────────────────────────────
# ECS Task: Order Receiver
# ──────────────────────────────────────────
resource "aws_ecs_task_definition" "receiver" {
  family                   = "hw7-receiver"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = "256"
  memory                   = "512"
  execution_role_arn       = data.aws_iam_role.lab_role.arn
  task_role_arn            = data.aws_iam_role.lab_role.arn

  container_definitions = jsonencode([{
    name      = "receiver"
    image     = "${data.aws_ecr_repository.hw7.repository_url}:latest"
    essential = true
    portMappings = [{ containerPort = 8080, protocol = "tcp" }]
    environment = [
      { name = "APP_MODE",      value = "receiver" },
      { name = "SNS_TOPIC_ARN", value = aws_sns_topic.orders.arn },
      { name = "SQS_QUEUE_URL", value = aws_sqs_queue.orders.url }
    ]
    logConfiguration = {
      logDriver = "awslogs"
      options = {
        awslogs-group         = "/ecs/hw7-receiver"
        awslogs-region        = "us-west-2"
        awslogs-stream-prefix = "ecs"
        awslogs-create-group  = "true"
      }
    }
  }])
}

resource "aws_ecs_service" "receiver" {
  name            = "hw7-receiver"
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.receiver.arn
  desired_count   = 1
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = [aws_subnet.public_1.id, aws_subnet.public_2.id]
    security_groups  = [aws_security_group.ecs.id]
    assign_public_ip = true
  }

  load_balancer {
    target_group_arn = aws_lb_target_group.receiver.arn
    container_name   = "receiver"
    container_port   = 8080
  }

  depends_on = [aws_lb_listener.main]
}

# ──────────────────────────────────────────
# ECS Task: Order Processor
# ──────────────────────────────────────────
resource "aws_ecs_task_definition" "processor" {
  family                   = "hw7-processor"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = "256"
  memory                   = "512"
  execution_role_arn       = data.aws_iam_role.lab_role.arn
  task_role_arn            = data.aws_iam_role.lab_role.arn

  container_definitions = jsonencode([{
    name      = "processor"
    image     = "${data.aws_ecr_repository.hw7.repository_url}:latest"
    essential = true
    environment = [
      { name = "APP_MODE",      value = "processor" },
      { name = "SQS_QUEUE_URL", value = aws_sqs_queue.orders.url },
      { name = "NUM_WORKERS",   value = var.num_workers }
    ]
    logConfiguration = {
      logDriver = "awslogs"
      options = {
        awslogs-group         = "/ecs/hw7-processor"
        awslogs-region        = var.aws_region
        awslogs-stream-prefix = "ecs"
        awslogs-create-group  = "true"
      }
    }
  }])
}

resource "aws_ecs_service" "processor" {
  name            = "hw7-processor"
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.processor.arn
  desired_count   = 1
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = [aws_subnet.public_1.id, aws_subnet.public_2.id]
    security_groups  = [aws_security_group.ecs.id]
    assign_public_ip = true
  }
}