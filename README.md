# startpage

generate a startpage of links for a static site hosted via AWS

## What It Is

- Lambda function that parses a bunch of RSS feeds, writes links to new posts from those feeds to an html template file, and uploads that file to a specific key in the S3 bucket hosting a static site
  + https://ginglis.me/start

### What Could Be Better

- generalized stack, more cloud providers (I'm really only experienced with AWS)
- create `scripts/create.sh`, a SAM file, CFN template, whatever that automates initial creation of the stack (Lambda {IAM role, environment variables}, Eventbridge rule) (assuming user already has Cloudfront, S3, etc configured for existing static site)
- trigger Lambda dynamically / on every GET with API Gateway, rather than at fixed rate
  + however I like to KISS, keep it simple stupid :) no need to overengineer
- some kind of testing

## How to Use It

**note**: the feeds and startpage are not generalized - they are what I personally use. If you wish to you this I'd recommend fork -> update these to your liking.

1. Create a Lambda function. 
2. Give the Lambda write access to the static site's S3 bucket.
3. Give it some environment variables:

    ```bash
    S3_BUCKET_REGION=<aws_region>
    S3_BUCKET=<bucket_name>
    S3_FILE_KEY=<startpage_uri>
    ```

4. Configure ~~Cloudwatch~~ Eventbridge to call the function at whatever rate you wish 
  + (I set mine to hourly, which is still within the free tier limits. As I get more feeds I'll decrease to daily gradually, since I can't read everything in a day)
5. After any updates to the template, feeds, or lambda function, run `scripts/update.sh`
