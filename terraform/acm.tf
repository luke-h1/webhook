resource "aws_acm_certificate" "webhook" {
  private_key       = var.private_key
  certificate_body  = var.certificate_body
  certificate_chain = var.certificate_chain

  tags = merge(var.tags, {
    Name        = local.domain_name
    Environment = var.env
  })
}
