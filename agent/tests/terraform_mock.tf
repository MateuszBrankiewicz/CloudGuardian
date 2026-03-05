resource "aws_s3_bucket" "prod_data" {
  bucket = "cloudguardian-prod-data"
  acl    = "public-read"

  tags = {
    Environment = "production"
    Sensitive   = "true"
  }
}

resource "aws_db_instance" "main_db" {
  identifier           = "cloudguardian-db"
  allocated_storage    = 20
  engine               = "postgres"
  instance_class       = "db.t3.micro"
  publicly_accessible  = false
}

resource "aws_s3_bucket" "private_logs" {
  bucket = "cloudguardian-logs"
  acl    = "private"
}
