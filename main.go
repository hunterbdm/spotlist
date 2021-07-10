package main

import (
	"bufio"
	"context"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/hunterbdm/hello-requests"
	"log"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type YTVideoData struct {
	ID string
	URL string
	Title string
	Channel string
	Views int
}

type Config struct {
	PlaylistUrl string
	PlaylistID string
	PreferLyrics bool
	YTPlaylistTitle string
	Workers int
}

func pullSpotifySongs(playlistID string) ([]string, error){
	jar := request.Jar()

	resp, err := request.Do(request.Options{
		URL: "https://open.spotify.com/playlist/" + playlistID + "?nd=1",
		Headers: request.Headers{
			"sec-ch-ua": "\" Not;A Brand\";v=\"99\", \"Google Chrome\";v=\"91\", \"Chromium\";v=\"91\"",
			"accept": "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.9",
			"dnt": "1",
			"upgrade-insecure-requests": "1",
			"sec-ch-ua-mobile": "?0",
			"user-agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
			"sec-fetch-site": "same-origin",
			"sec-fetch-mode": "same-origin",
			"sec-fetch-dest": "empty",
			"accept-encoding": "gzip, deflate, br",
			"accept-language": "en-US,en;q=0.9",
		},
		HeaderOrder: request.HeaderOrder{
			"sec-ch-ua",
			"accept",
			"dnt",
			"upgrade-insecure-requests",
			"sec-ch-ua-mobile",
			"user-agent",
			"sec-fetch-site",
			"sec-fetch-mode",
			"sec-fetch-dest",
			"accept-encoding",
			"accept-language",
			"cookie",
		},
		Jar: jar,
	})

	if err != nil {
		return nil, err
	} else if resp.StatusCode != 200 {
		return nil, errors.New("invalid response code " + strconv.Itoa(resp.StatusCode))
	}

	matcher, _ := regexp.Compile("accessToken\":\"[^\"]+")
	authTokens := strings.Replace(matcher.FindString(resp.Body), "accessToken\":\"", "", 1)

	var songs []string
	var pullPage func(int) error

	pullPage = func(page int) error {
		resp, err = request.Do(request.Options{
			URL: "https://api.spotify.com/v1/playlists/" + playlistID + "/tracks?offset=" + strconv.Itoa(100 * page) +"&limit=100&additional_types=track%2Cepisode&market=US",
			Headers: request.Headers{
				"sec-ch-ua": "\" Not;A Brand\";v=\"99\", \"Google Chrome\";v=\"91\", \"Chromium\";v=\"91\"",
				"dnt": "1",
				"accept-language": "en",
				"sec-ch-ua-mobile": "?0",
				"authorization": "Bearer " + authTokens,
				"accept": "application/json",
				"user-agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
				"spotify-app-version": "0.0.0.0-unknown",
				"app-platform": "WebPlayer",
				"origin": "https://open.spotify.com",
				"sec-fetch-site": "same-site",
				"sec-fetch-mode": "cors",
				"sec-fetch-dest": "empty",
				"referer": "https://open.spotify.com/",
				"accept-encoding": "gzip, deflate, br",
			},
			HeaderOrder: request.HeaderOrder{
				"sec-ch-ua",
				"dnt",
				"accept-language",
				"sec-ch-ua-mobile",
				"authorization",
				"accept",
				"user-agent",
				"spotify-app-version",
				"app-platform",
				"origin",
				"sec-fetch-site",
				"sec-fetch-mode",
				"sec-fetch-dest",
				"referer",
				"accept-encoding",
				"cookie",
			},
			Jar: jar,
		})

		if err != nil {
			return err
		}
		if resp.StatusCode != 200 {
			return errors.New("invalid API response code " + strconv.Itoa(resp.StatusCode))
		}

		playlistData := struct {
			Items []struct{
				Track struct{
					URL string `json:"href"`
					ID string `json:"id"`
					Name string `json:"name"`
					ExternalURLs struct{
						Spotify string `json:"spotify"`
					} `json:"external_urls"`
					Artists []struct{
						URL string `json:"href"`
						ID string `json:"id"`
						Name string `json:"name"`
						ExternalURLs struct{
							Spotify string `json:"spotify"`
						} `json:"external_urls"`
					} `json:"artists"`
				} `json:"track"`
			} `json:"items"`
		}{}
		err = json.Unmarshal([]byte(resp.Body), &playlistData)
		if err != nil {
			return errors.New("failed parsing API json response")
		}

		for i := range playlistData.Items {
			songData := playlistData.Items[i].Track
			songs = append(songs, songData.Name + " (" + songData.Artists[0].Name + ")")
		}

		if len(playlistData.Items) == 100 {
			return pullPage(page+1)
		}

		return nil
	}
	err = pullPage(0)
	if err != nil {
		return nil, err
	}

	return songs, nil
}

func youtubeSearch(songTitle string) ([]YTVideoData, error){
	matchJsonRegex, _ := regexp.Compile("var ytInitialData = .+?;<\\/script>")

	resp, err := request.Do(request.Options{
		URL: "https://www.youtube.com/results?" + url.Values{
			"search_query": []string{songTitle},
		}.Encode(),
		Headers: request.Headers{
			"sec-ch-ua": "\" Not;A Brand\";v=\"99\", \"Google Chrome\";v=\"91\", \"Chromium\";v=\"91\"",
			"sec-ch-ua-mobile": "?0",
			"upgrade-insecure-requests": "1",
			"user-agent": "	Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
			"accept": "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.9",
			//"x-client-data": "CJS2yQEIo7bJAQjEtskBCKmdygEI8+HKAQjQmssBCKCgywEIrfLLAQjc8ssBCO/yywEIkPfLAQiz+MsBCJb5ywEInvnLAQj4+csBGLryywEY3/nLAQ==",
			"sec-fetch-site": "none",
			"sec-fetch-mode": "navigate",
			"sec-fetch-user": "?1",
			"sec-fetch-dest": "document",
			"accept-encoding": "gzip, deflate, br",
			"accept-language": "en-US,en;q=0.9",
		},
		HeaderOrder: request.HeaderOrder{
			"sec-ch-ua",
			"sec-ch-ua-mobile",
			"upgrade-insecure-requests",
			"user-agent",
			"accept",
			"sec-fetch-site",
			"sec-fetch-mode",
			"sec-fetch-user",
			"sec-fetch-dest",
			"accept-encoding",
			"accept-language",
			"cookie",
		},
	})

	if err != nil {
		return nil, err
	} else if resp.StatusCode != 200 {
		return nil, errors.New("bad response from YT: " + strconv.Itoa(resp.StatusCode))
	}

	jsonFromBody := matchJsonRegex.FindString(resp.Body)
	jsonFromBody = jsonFromBody[20:len(jsonFromBody)-10]

	ytSearchData := struct {
		Contents struct{
			TwoColumnSearchResultsRenderer struct{
				PrimaryContents struct{
					SectionListRenderer struct{
						Contents []struct{
							ItemSectionRenderer struct{
								Contents []struct{
									VideoRenderer *struct{
										VideoID string `json:"videoId"`
										Title struct{
											Runs []struct{
												Text string `json:"text"`
											} `json:"runs"`
										} `json:"title"`
										OwnerText struct{
											Runs []struct{
												Text string `json:"text"`
											} `json:"runs"`
										} `json:"ownerText"`
										ViewCountText struct{
											SimpleText string `json:"simpleText"`
										} `json:"viewCountText"`
									} `json:"videoRenderer"`
								} `json:"contents"`
							} `json:"itemSectionRenderer"`
						} `json:"contents"`
					} `json:"sectionListRenderer"`
				} `json:"primaryContents"`
			} `json:"twoColumnSearchResultsRenderer"`
		} `json:"contents"`
	}{}
	err = json.Unmarshal([]byte(jsonFromBody), &ytSearchData)
	if err != nil {
		return nil, err
	}

	var cleanData []YTVideoData
	for i := range ytSearchData.Contents.TwoColumnSearchResultsRenderer.PrimaryContents.SectionListRenderer.Contents[0].ItemSectionRenderer.Contents {
		vidData := ytSearchData.Contents.TwoColumnSearchResultsRenderer.PrimaryContents.SectionListRenderer.Contents[0].ItemSectionRenderer.Contents[i]
		// Skip non videos
		if vidData.VideoRenderer == nil {
			continue
		}

		views, _ := strconv.Atoi(strings.ReplaceAll(strings.Replace(vidData.VideoRenderer.ViewCountText.SimpleText, " views", "", 1), ",", ""))
		cleanData = append(cleanData, YTVideoData{
			ID:      vidData.VideoRenderer.VideoID,
			URL:     "https://www.youtube.com/watch?v=" + vidData.VideoRenderer.VideoID,
			Title:   vidData.VideoRenderer.Title.Runs[0].Text,
			Channel: vidData.VideoRenderer.OwnerText.Runs[0].Text,
			Views:   views,
		})
	}

	return cleanData, nil
}

func googleLogin(requiredCookies []string) ([]*network.Cookie, error) {
	// Create context (browser)
	opts := append(chromedp.DefaultExecAllocatorOptions[3:22],
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.WindowSize(600, 600),
	)
	ctx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel = chromedp.NewContext(ctx)
	defer cancel()

	err := chromedp.Run(ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			_, err := page.AddScriptToEvaluateOnNewDocument("delete navigator.__proto__.webdriver;").Do(ctx)
			if err != nil {
				return err
			}
			return nil
		}),
		chromedp.Navigate("https://accounts.google.com/signin/v2/identifier?service=youtube&continue=https%3A%2F%2Fwww.youtube.com%2Fsignin%3Faction_handle_signin%3Dtrue%26app%3Ddesktop%26hl%3Den%26next%3Dhttps%253A%252F%252Fwww.youtube.com%252F"))
	if err != nil {
		return nil, err
	}

	var cookies []*network.Cookie
	for true {
		cookies, err = network.GetCookies().WithUrls([]string{
			"https://www.youtube.com/youtubei/v1/playlist/create",
		}).Do(cdp.WithExecutor(ctx, chromedp.FromContext(ctx).Target))
		if err != nil {
			return nil, err
		}

		hasAllCookies := true
		for c := range requiredCookies {
			requiredName := requiredCookies[c]

			hasCookie := false
			for i := range cookies {
				if cookies[i].Name == requiredName {
					hasCookie = true
					break
				}
			}

			if !hasCookie {
				hasAllCookies = false
				break
			}
		}

		if hasAllCookies {
			break
		}
		time.Sleep(time.Millisecond * 200)
	}

	return cookies, nil
}

func createPlaylistYT(title string, videoIDs []string, cookies []*network.Cookie) (*string, error) {
	/*
	SAPISIDHASH 1625901976_c590075089cc08a8367475d4d528eb0508687b86

	hash (sha1):

	${timestamp} ${SAPISID} ${origin}
	1625901976 jVJ7JQV3IaxWp1Q0/AflKp6v7HSShk6RQm https://www.youtube.com
	c590075089cc08a8367475d4d528eb0508687b86

	SAPISIDHASH 1625901976_c590075089cc08a8367475d4d528eb0508687b86
	 */

	now := strconv.Itoa(int(time.Now().UnixNano() / int64(time.Second)))
	hash := now + " "

	// Build cookie string
	cookieHeader := ""
	for i := range cookies {
		c := cookies[i]
		if i > 0 {
			cookieHeader += " "
		}
		cookieHeader += c.Name + "=" + c.Value + ";"

		if c.Name == "SAPISID" {
			hash += c.Value
		}
	}

	hash += " https://www.youtube.com"
	sha := sha1.New()
	sha.Write([]byte(hash))
	hash = fmt.Sprintf("%x", sha.Sum(nil))

	resp, err := request.Do(request.Options{
		Method: "POST",
		URL: "https://www.youtube.com/youtubei/v1/playlist/create?key=AIzaSyAO_FJ2SlqU8Q4STEHLGCilw_Y9_11qcW8",
		Headers: request.Headers{
			"sec-ch-ua": "\" Not;A Brand\";v=\"99\", \"Google Chrome\";v=\"91\", \"Chromium\";v=\"91\"",
			"x-origin": "https://www.youtube.com",
			"sec-ch-ua-mobile": "?0",
			"authorization": "SAPISIDHASH " + now + "_" + hash,
			"content-type": "application/json",
			"user-agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
			"accept": "*/*",
			"origin": "https://www.youtube.com",
			"sec-fetch-site": "same-origin",
			"sec-fetch-mode": "same-origin",
			"sec-fetch-dest": "empty",
			"referer": "https://www.youtube.com/watch?v=" + videoIDs[0],
			"accept-encoding": "gzip, deflate, br",
			"accept-language": "en-US,en;q=0.9",
			"cookie": cookieHeader,
		},
		HeaderOrder: request.HeaderOrder{
			"content-length",
			"sec-ch-ua",
			"x-origin",
			"sec-ch-ua-mobile",
			"authorization",
			"content-type",
			"user-agent",
			"accept",
			"origin",
			"sec-fetch-site",
			"sec-fetch-mode",
			"sec-fetch-dest",
			"referer",
			"accept-encoding",
			"accept-language",
			"cookie",
		},
		Json: request.JSON{
			"context": request.JSON{
				"client": request.JSON{
					"clientName": "WEB",
					"clientVersion": "2.20210708.06.00",
				},
			},
			"title": title,
			"privacyStatus": "UNLISTED",
			"videoIds": videoIDs,
		},
	})

	if err != nil {
		return nil, err
	} else if resp.StatusCode != 200 {
		return nil, errors.New("bad response " + strconv.Itoa(resp.StatusCode))
	}

	res := struct {
		PlaylistID string `json:"playlistId"`
	}{}
	_ = json.Unmarshal([]byte(resp.Body), &res)

	ytPlaylistUrl := "https://www.youtube.com/playlist?list=" + res.PlaylistID

	return &ytPlaylistUrl, nil
}

/* https://gist.github.com/hyg/9c4afcd91fe24316cbf0 */
func openbrowser(url string) {
	var err error

	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	config := Config{
		Workers:         50,
	}
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("Spotify to Youtube playlist (Made by @hunter_bdm)")

	fmt.Print("Enter Spotify playlist URL: ")
	config.PlaylistUrl, _ = reader.ReadString('\n')
	config.PlaylistUrl = strings.Replace(config.PlaylistUrl, "\n", "", -1)

	var prefLyricsInput string
	fmt.Print("Prefer lyrics(y/n): ")
	prefLyricsInput, _ = reader.ReadString('\n')
	prefLyricsInput = strings.Replace(prefLyricsInput, "\n", "", -1)
	if strings.TrimSpace(strings.ToLower(prefLyricsInput)) == "y" {
		config.PreferLyrics = true
	}

	fmt.Print("Enter YouTube playlist name: ")
	config.YTPlaylistTitle, _ = reader.ReadString('\n')
	config.YTPlaylistTitle = strings.Replace(config.YTPlaylistTitle, "\n", "", -1)

	// Find playlist ID from playlist URL
	playlistIDRegex, _ := regexp.Compile("\\/playlist\\/[^\\/?]+")
	config.PlaylistID = strings.Replace(playlistIDRegex.FindString(config.PlaylistUrl), "/playlist/", "", 1)

	fmt.Println("Pulling songs from Spotify playlist")

	// Get song names
	songs, err := pullSpotifySongs(config.PlaylistID)
	if err != nil {
		fmt.Println("Failed pulling spotify playlist:", err)
		return
	}

	fmt.Println("Found", len(songs), "songs on the Spotify playlist")
	fmt.Println("Searching for youtube equivalents...")

	var ytSongsToAdd []string
	for i := 0; i < config.Workers; i++ {
		worker := func() {
			for len(songs) > 0 {
				var songName string
				songName, songs = songs[0], songs[1:]

				// Get youtube results for song name
				results, err := youtubeSearch(songName)
				if err != nil {
					fmt.Println("Failed YT search for", songName, err)
					continue
				}

				// Pick best result according to preferences
				var picked *YTVideoData
				if config.PreferLyrics {
					for i := range results {
						if strings.Contains(strings.ToLower(results[i].Title), "lyrics") {
							picked = &results[i]
							break
						}
					}
				}
				if picked == nil {
					picked = &results[0]
				}

				ytSongsToAdd = append(ytSongsToAdd, picked.ID)
				//fmt.Println("FOUND", picked.Title, "@", picked.URL)
			}
		}

		// Dont run the last worker in a goroutine so the code below continues after all videos are fetched
		if i == config.Workers-1 {
			worker()
		} else {
			go worker()
		}
	}

	fmt.Println("Done searching, waiting for Google login...")

	googleCookies, err := googleLogin([]string{
		"__Secure-3PSIDCC",
		"__Secure-3PAPISID",
		"__Secure-1PAPISID",
		"__Secure-1PSID",
		"__Secure-3PSID",
		"LOGIN_INFO",
		"SIDCC",
		"PREF",
		"SAPISID",
		"APISID",
		"SSID",
		"SID",
		"HSID",
		"YSC",
	})
	if err != nil {
		fmt.Println("Failed google login:", err)
		return
	}

	fmt.Println("Login complete, attempting to create playlist with", len(ytSongsToAdd), "videos")

	newPlaylistUrl, err := createPlaylistYT(config.YTPlaylistTitle, ytSongsToAdd, googleCookies)
	if err != nil {
		fmt.Println("Failed creating playlist:", err)
		return
	}

	fmt.Println("Created Playlist:", *newPlaylistUrl)
	openbrowser(*newPlaylistUrl)

	return
}