# Local Github Action Testing

1. Install [Act](https://github.com/nektos/act)

1. Create a `.secrets` file with the following content:

   ```bash
    BOT_TOKEN=your_github_token
    GITHUB_REPOSITORY_NAME=warpgate
    GITHUB_REPOSITORY_OWNER=cowdogmoo
   ```

1. Uncomment relevant sections in
   `.github/workflows/dockerfile-image-builder.yaml` and run the following command:

   ```bash
   act -W .github/workflows/dockerfile-image-builder.yaml --secret-file .secrets
   ```

You will know the action has run successfully if you see the following output:

```bash
[Build, Publish, and Test Container Images/build-and-push] 🏁  Job succeeded
```
