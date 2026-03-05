# PROBLEM 1: Gigantyczna instancja do prostych zadań (FinOps Leak)
resource "aws_instance" "legacy_app_server" {
  instance_type = "m5.4xlarge" # Koszt ok. $550/msc
  ami           = "ami-12345678"
  tags = {
    Name = "Legacy-App-Dev" # Słowo "Dev" powinno zasugerować AI redukcję kosztów
  }
}

# PROBLEM 2: Publiczny Bucket z danymi (Security Nightmare)
resource "aws_s3_bucket" "user_uploads_backup" {
  bucket = "cloudguardian-public-leak-test"
  acl    = "public-read" # KRYTYCZNE: Publiczny dostęp
}

# PROBLEM 3: Baza danych bez szyfrowania i bez backupu (Compliance Issue)
resource "aws_db_instance" "hr_portal_db" {
  instance_class       = "db.t3.xlarge"
  engine               = "mysql"
  publicly_accessible  = true
  storage_encrypted    = false # BRAK SZYFROWANIA
  backup_retention_period = 0  # BRAK BACKUPU
}
