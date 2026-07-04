output "waf_arn" {
  value       = aws_wafv2_web_acl.main.arn
  description = "WAF Web ACL ARN — pass to ALB module"
}
