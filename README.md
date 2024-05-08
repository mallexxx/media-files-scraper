Media Files Scraper
===================

Media Files Scraper is a GoLang project designed to automate the process of organizing and scraping metadata for movies and TV series files. It checks specified directories for media files, scrapes information about them, and organizes them for use in media libraries like Kodi.

Features
--------

*   **Automated Scraping**: The project automatically scrapes metadata for movie and TV series files.
*   **Torrent Data Integration**: It attempts to match media files against torrent data from the Transmission client API to retrieve IMDb ID or movie title from originating Rutracker topics.
*   **Database Querying**: If torrent data is unavailable, it queries TMDB, IMDb, and Kinopoisk databases to guess correct movie/series names.
*   **Integration with ChatGPT**: It utilizes ChatGPT to clean up movie names if needed.
*   **Cross-Platform**: The project is written in GoLang, making it cross-platform compatible.

Configuration
-------------

Configuration is done using `config.json`, with a default configuration provided in `config.default.json`. To set up the project:

1.  Add your API tokens (`tmdb_api_key`, `openai_api_key`, `kinopoisk_api_key`) in the `config.json` file.
2.  Set the Transmission RPC URL under `"transmission"` if you are using Transmission client.
3.  Add directories to scan in the `"directories"` array.
4.  Specify directories to create symlinks for Kodi under `"output"`.

Usage
-----

1.  Clone the repository:
    
    `git clone https://github.com/yourusername/media-files-scraper.git`
    
2.  Install dependencies:
    
    `go mod tidy`
    
3.  Build the project:
    
    `go build`
    
4.  Run the executable:
    
    `./media-files-scraper`
    

Contributing
------------

Contributions are welcome! If you encounter any issues or have suggestions for improvements, please open an issue or submit a pull request.

License
-------

This project is licensed under the MIT License. See the LICENSE file for details.
