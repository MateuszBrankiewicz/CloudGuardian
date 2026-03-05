resource "aws_vpc" "main" {
  cidr_block = "10.0.0.0/16"
}

resource "aws_s3_bucket" "sensitive_data" {
  bucket = "company-private-pesel-storage"
  # To pole powinno zostać wyłapane przez nasz parser
  tags = {
    Environment = "Prod"
    Owner       = "Security"
  }
}

resource "aws_db_instance" "database" {
  instance_class = "db.t3.micro"
  allocated_storage = 20
  # Zależność, którą powinien wyłapać graf:
  db_subnet_group_name = aws_vpc.main.id 
}
