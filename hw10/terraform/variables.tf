variable "aws_region" {
  default = "us-west-2"
}

variable "lf_w" {
  description = "Write quorum for leader-follower"
  default     = "5"
}

variable "lf_r" {
  description = "Read quorum for leader-follower"
  default     = "1"
}