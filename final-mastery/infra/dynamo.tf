resource "aws_dynamodb_table" "albums" {
  name         = "Albums"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "album_id"

  attribute {
    name = "album_id"
    type = "S"
  }

  tags = { Name = "${var.app_name}-albums" }
}

resource "aws_dynamodb_table" "photos" {
  name         = "Photos"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "photo_id"

  attribute {
    name = "photo_id"
    type = "S"
  }

  tags = { Name = "${var.app_name}-photos" }
}