# FTC Parts Spider

This program runs a spider over the multiple vendor websites including: Pitsco, AndyMark, goBILDA, ServoCity, REV Robotics and Studica in order to generate a full list of their products to ensure that all the models have been created for them.

## `token.json` initialization

In order to access the Google sheets documents that holds the parts library organization, you will need to have a local `token.json` file.  If you don't have it, the first time that you win `ftc_parts_spider`, it will prompt you with:

```TEXT
────────────────────────────────────────────────────────
🔐 To authorize access to Google Sheets:
1. Open the following URL in your browser:

  https://accounts.google.com/o/oauth2/auth?access_type=offline&client_id=xxxxxxxxxxxx-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx.apps.googleusercontent.com&redirect_uri=http%3A%2F%2Flocalhost&response_type=code&scope=https%3A%2F%2Fwww.googleapis.com%2Fauth%2Fspreadsheets.readonly&state=state-token

2. Copy and paste the code you receive below:
────────────────────────────────────────────────────────
Enter the authorization code:
```

You will have to Choose an account that has access to the parts library spreadsheets, then click on a page that says that the app is not verified for which you need to click `Advanced` and then `Go to Quickstart (unsafe)` followed by a dialog indicating that Quickstart wants access to the document.  Click Continue and it will take you to a file that doesn't exist.  It will be something like

```TEXT
http://localhost/?state=state-token&code=xxxxxxx-xxxxxxxxxxxx-xxxxxxxxxxx-xxxxxxxxxxxxxxxxxxx_xxxxxxxxxxxxxxxxxxxx&scope=https://www.googleapis.com/auth/spreadsheets.readonly
```

select the code value (everything between `code=` and `&scope=`) and then paste that into the prompt for the `ftc_parts_spider` to continue.

You should only have to do this once as it will store the value in a local file called `token.json`.

## Running

1. `git clone` into a local directory
2. `go build` to create the executable
3. `ftc_parts_spider -target <vendor>` will take a while to run but will create a file called `vendor.txt`.
4. Import that `vendor.txt` file into the corresponding spreadsheet using the \` character as a separator.
