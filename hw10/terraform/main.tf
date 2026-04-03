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

# ECR Repositories

resource "aws_ecr_repository" "kv_lf" {
  name         = "kv-leader-follower"
  force_delete = true
}

resource "aws_ecr_repository" "kv_ll" {
  name         = "kv-leaderless"
  force_delete = true
}

# VPC

resource "aws_vpc" "main" {
  cidr_block           = "10.0.0.0/16"
  enable_dns_support   = true
  enable_dns_hostnames = true
  tags = { Name = "hw10-vpc" }
}

resource "aws_internet_gateway" "main" {
  vpc_id = aws_vpc.main.id
  tags   = { Name = "hw10-igw" }
}

resource "aws_subnet" "public_1" {
  vpc_id                  = aws_vpc.main.id
  cidr_block              = "10.0.1.0/24"
  availability_zone       = "us-west-2a"
  map_public_ip_on_launch = true
  tags                    = { Name = "hw10-public-1" }
}

resource "aws_subnet" "public_2" {
  vpc_id                  = aws_vpc.main.id
  cidr_block              = "10.0.2.0/24"
  availability_zone       = "us-west-2b"
  map_public_ip_on_launch = true
  tags                    = { Name = "hw10-public-2" }
}

resource "aws_route_table" "public" {
  vpc_id = aws_vpc.main.id
  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.main.id
  }
  tags = { Name = "hw10-rt" }
}

resource "aws_route_table_association" "public_1" {
  subnet_id      = aws_subnet.public_1.id
  route_table_id = aws_route_table.public.id
}

resource "aws_route_table_association" "public_2" {
  subnet_id      = aws_subnet.public_2.id
  route_table_id = aws_route_table.public.id
}

# Security Group

resource "aws_security_group" "kv_nodes" {
  name   = "hw10-kv-nodes"
  vpc_id = aws_vpc.main.id

  ingress {
    from_port   = 8080
    to_port     = 8080
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = { Name = "hw10-kv-sg" }
}

# IAM - use existing LabRole from AWS Academy

data "aws_iam_role" "lab_role" {
  name = "LabRole"
}

# ECS Cluster

resource "aws_ecs_cluster" "main" {
  name = "hw10-cluster"
}

# Leader-Follower Task Definitions

resource "aws_ecs_task_definition" "lf_leader" {
  family                   = "hw10-lf-leader"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = "256"
  memory                   = "512"
  execution_role_arn       = data.aws_iam_role.lab_role.arn

  container_definitions = jsonencode([{
    name      = "lf-leader"
    image     = "${aws_ecr_repository.kv_lf.repository_url}:latest"
    essential = true
    portMappings = [{ containerPort = 8080, protocol = "tcp" }]
    environment = [
      { name = "ROLE",  value = "leader" },
      { name = "W",     value = var.lf_w },
      { name = "R",     value = var.lf_r },
      { name = "NODE_ADDRESSES", value = "" }
    ]
    logConfiguration = {
      logDriver = "awslogs"
      options = {
        awslogs-group         = "/ecs/hw10-lf-leader"
        awslogs-region        = var.aws_region
        awslogs-stream-prefix = "ecs"
        awslogs-create-group  = "true"
      }
    }
  }])
}

resource "aws_ecs_task_definition" "lf_follower" {
  family                   = "hw10-lf-follower"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = "256"
  memory                   = "512"
  execution_role_arn       = data.aws_iam_role.lab_role.arn

  container_definitions = jsonencode([{
    name      = "lf-follower"
    image     = "${aws_ecr_repository.kv_lf.repository_url}:latest"
    essential = true
    portMappings = [{ containerPort = 8080, protocol = "tcp" }]
    environment = [
      { name = "ROLE",           value = "follower" },
      { name = "W",              value = "1" },
      { name = "R",              value = "1" },
      { name = "NODE_ADDRESSES", value = "" }
    ]
    logConfiguration = {
      logDriver = "awslogs"
      options = {
        awslogs-group         = "/ecs/hw10-lf-follower"
        awslogs-region        = var.aws_region
        awslogs-stream-prefix = "ecs"
        awslogs-create-group  = "true"
      }
    }
  }])
}

resource "aws_ecs_task_definition" "ll_node" {
  family                   = "hw10-ll-node"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = "256"
  memory                   = "512"
  execution_role_arn       = data.aws_iam_role.lab_role.arn

  container_definitions = jsonencode([{
    name      = "ll-node"
    image     = "${aws_ecr_repository.kv_ll.repository_url}:latest"
    essential = true
    portMappings = [{ containerPort = 8080, protocol = "tcp" }]
    environment = [
      { name = "NODE_ID",        value = "" },
      { name = "NODE_ADDRESSES", value = "" }
    ]
    logConfiguration = {
      logDriver = "awslogs"
      options = {
        awslogs-group         = "/ecs/hw10-ll-node"
        awslogs-region        = var.aws_region
        awslogs-stream-prefix = "ecs"
        awslogs-create-group  = "true"
      }
    }
  }])
}

# ECS Services - Leader Follower

resource "aws_ecs_service" "lf_leader" {
  name            = "hw10-lf-leader"
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.lf_leader.arn
  desired_count   = 1
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = [aws_subnet.public_1.id, aws_subnet.public_2.id]
    security_groups  = [aws_security_group.kv_nodes.id]
    assign_public_ip = true
  }
}

resource "aws_ecs_service" "lf_followers" {
  name            = "hw10-lf-followers"
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.lf_follower.arn
  desired_count   = 4
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = [aws_subnet.public_1.id, aws_subnet.public_2.id]
    security_groups  = [aws_security_group.kv_nodes.id]
    assign_public_ip = true
  }
}

# ECS Services - Leaderless

resource "aws_ecs_service" "ll_nodes" {
  name            = "hw10-ll-nodes"
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.ll_node.arn
  desired_count   = 5
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = [aws_subnet.public_1.id, aws_subnet.public_2.id]
    security_groups  = [aws_security_group.kv_nodes.id]
    assign_public_ip = true
  }
}