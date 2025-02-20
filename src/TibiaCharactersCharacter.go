package main

import (
	"fmt"
	"log"
	"net/http"
	"reflect"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/TibiaData/tibiadata-api-go/src/validation"
	//"time"
)

// Child of CharacterInfo
type Houses struct {
	Name    string `json:"name"`    // The name of the house.
	Town    string `json:"town"`    // The town where the house is located in.
	Paid    string `json:"paid"`    // The date the last paid rent is due.
	HouseID int    `json:"houseid"` // The internal ID of the house.
}

// Child of CharacterInfo
type CharacterGuild struct {
	GuildName string `json:"name,omitempty"` // The name of the guild.
	Rank      string `json:"rank,omitempty"` // The character's rank in the guild.
}

// Child of Character
type CharacterInfo struct {
	Name              string         `json:"name"`                    // The name of the character.
	FormerNames       []string       `json:"former_names,omitempty"`  // List of former names of the character.
	Traded            bool           `json:"traded,omitempty"`        // Whether the character was traded. (last 6 months)
	DeletionDate      string         `json:"deletion_date,omitempty"` // The date when the character will be deleted. (if scheduled for deletion)
	Sex               string         `json:"sex"`                     // The character's sex.
	Title             string         `json:"title"`                   // The character's selected title.
	UnlockedTitles    int            `json:"unlocked_titles"`         // The number of titles the character has unlocked.
	Vocation          string         `json:"vocation"`                // The character's vocation.
	Level             int            `json:"level"`                   // The character's level.
	AchievementPoints int            `json:"achievement_points"`      // The total of achievement points the character has.
	World             string         `json:"world"`                   // The character's current world.
	FormerWorlds      []string       `json:"former_worlds,omitempty"` // List of former worlds the character was in. (last 6 months)
	Residence         string         `json:"residence"`               // The character's current residence.
	MarriedTo         string         `json:"married_to,omitempty"`    // The name of the character's husband/spouse.
	Houses            []Houses       `json:"houses,omitempty"`        // List of houses the character owns currently.
	Guild             CharacterGuild `json:"guild"`                   // The guild that the character is member of.
	LastLogin         string         `json:"last_login,omitempty"`    // The character's last logged in time.
	Position          string         `json:"position,omitempty"`      // The character's special position.
	AccountStatus     string         `json:"account_status"`          // Whether account is Free or Premium.
	Comment           string         `json:"comment,omitempty"`       // The character's comment.
}

// Child of Character
type AccountBadges struct {
	Name        string `json:"name"`        // The name of the badge.
	IconURL     string `json:"icon_url"`    // The URL to the badge's icon.
	Description string `json:"description"` // The description of the badge.
}

// Child of Character
type Achievements struct {
	Name   string `json:"name"`   // The name of the achievement.
	Grade  int    `json:"grade"`  // The grade/stars of the achievement.
	Secret bool   `json:"secret"` // Whether it is a secret achievement or not.
}

// Child of Deaths
type Killers struct {
	Name   string `json:"name"`   // The name of the killer/assist.
	Player bool   `json:"player"` // Whether it is a player or not.
	Traded bool   `json:"traded"` // If the killer/assist was traded after the death.
	Summon string `json:"summon"` // The name of the summoned creature.
}

// Child of Character
type Deaths struct {
	Time    string    `json:"time"`    // The timestamp when the death occurred.
	Level   int       `json:"level"`   // The level when the death occurred.
	Killers []Killers `json:"killers"` // List of killers involved.
	Assists []Killers `json:"assists"` // List of assists involved.
	Reason  string    `json:"reason"`  // The plain text reason of death.
}

// Child of Character
type AccountInformation struct {
	Position     string `json:"position,omitempty"`      // The account's special position.
	Created      string `json:"created,omitempty"`       // The account's date of creation.
	LoyaltyTitle string `json:"loyalty_title,omitempty"` // The account's loyalty title.
}

// Child of Character
type OtherCharacters struct {
	Name     string `json:"name"`               // The name of the character.
	World    string `json:"world"`              // The name of the world.
	Status   string `json:"status"`             // The status of the character being online or offline.
	Deleted  bool   `json:"deleted"`            // Whether the character is scheduled for deletion or not.
	Main     bool   `json:"main"`               // Whether this is the main character or not.
	Traded   bool   `json:"traded"`             // Whether the character has been traded last 6 months or not.
	Position string `json:"position,omitempty"` // // The character's special position.
}

// Child of JSONData
type Character struct {
	CharacterInfo      CharacterInfo      `json:"character"`                     // The character's information.
	AccountBadges      []AccountBadges    `json:"account_badges,omitempty"`      // The account's badges.
	Achievements       []Achievements     `json:"achievements,omitempty"`        // The character's achievements.
	Deaths             []Deaths           `json:"deaths,omitempty"`              // The character's deaths.
	AccountInformation AccountInformation `json:"account_information,omitempty"` // The account information.
	OtherCharacters    []OtherCharacters  `json:"other_characters,omitempty"`    // The account's other characters.
}

// The base includes two levels, Characters and Information
type CharacterResponse struct {
	Character   Character   `json:"character"`
	Information Information `json:"information"`
}

// From https://pkg.go.dev/golang.org/x/net/html/atom
// This is an Atom. An Atom is an integer code for a string.
// Instead of importing the whole lib, we thought it would be
// best to just simply use the Br constant value.
const Br = 0x202

var (
	deathRegex               = regexp.MustCompile(`<td.*>(.*)<\/td><td>(.*) at Level ([0-9]+) by (.*).<\/td>`)
	summonRegex              = regexp.MustCompile(`(an? .+) of ([^<]+)`)
	accountBadgesRegex       = regexp.MustCompile(`\(this\), &#39;(.*)&#39;, &#39;(.*)&#39;,.*\).*src="(.*)" alt=.*`)
	accountAchievementsRegex = regexp.MustCompile(`<td class="[a-zA-Z0-9_.-]+">(.*)<\/td><td>(.*)?<?.*<\/td>`)
	titleRegex               = regexp.MustCompile(`(.*) \(([0-9]+).*`)
	characterInfoRegex       = regexp.MustCompile(`<td.*<nobr>[0-9]+\..(.*)<\/nobr><\/td><td.*><nobr>(.*)<\/nobr><\/td><td style="width: 70%">(.*)<\/td><td.*`)
)

// TibiaCharactersCharacter func
func TibiaCharactersCharacterImpl(BoxContentHTML string) (*CharacterResponse, error) {
	var (
		// local strings used in this function
		localDivQueryString = ".TableContentContainer tr"
		localTradedString   = " (traded)"

		// Declaring vars for later use..
		CharacterInfoData      CharacterInfo
		AccountBadgesData      []AccountBadges
		AchievementsData       []Achievements
		DeathsData             []Deaths
		AccountInformationData AccountInformation
		OtherCharactersData    []OtherCharacters

		// Errors
		characterNotFound bool
		insideError       error
	)

	// Loading HTML data into ReaderHTML for goquery with NewReader
	ReaderHTML, err := goquery.NewDocumentFromReader(strings.NewReader(BoxContentHTML))
	if err != nil {
		return nil, fmt.Errorf("TibiaCharactersCharacterImpl failed at  goquery.NewDocumentFromReader, err: %s", err)
	}

	// Running query on every .TableContainer
	ReaderHTML.Find(".TableContainer").EachWithBreak(func(index int, s *goquery.Selection) bool {
		if insideError != nil {
			return false
		}

		SectionTextQuery := s.Find("div.Text")

		SectionName := SectionTextQuery.Nodes[0].FirstChild.Data

		// query current node with goquery
		CharacterDivQuery := goquery.NewDocumentFromNode(s.Nodes[0])

		switch SectionName {
		case "Could not find character":
			characterNotFound = true
			return false
		case "Character Information", "Account Information":
			// Running query over each tr in character content container
			CharacterDivQuery.Find(localDivQueryString).Each(func(index int, s *goquery.Selection) {
				RowNameQuery := s.Find("td[class^='Label']")

				RowName := RowNameQuery.Nodes[0].FirstChild.Data
				RowData := RowNameQuery.Nodes[0].NextSibling.FirstChild.Data

				switch TibiaDataSanitizeStrings(RowName) {
				case "Name:":
					Tmp := strings.Split(RowData, "<")
					CharacterInfoData.Name = strings.TrimSpace(Tmp[0])
					if strings.Contains(Tmp[0], ", will be deleted at") {
						Tmp2 := strings.Split(Tmp[0], ", will be deleted at ")
						CharacterInfoData.Name = Tmp2[0]
						CharacterInfoData.DeletionDate = TibiaDataDatetime(strings.TrimSpace(Tmp2[1]))
					}
					if strings.Contains(RowData, localTradedString) {
						CharacterInfoData.Traded = true
						CharacterInfoData.Name = strings.Replace(CharacterInfoData.Name, localTradedString, "", -1)
					}
				case "Former Names:":
					CharacterInfoData.FormerNames = strings.Split(RowData, ", ")
				case "Sex:":
					CharacterInfoData.Sex = RowData
				case "Title:":
					subma1t := titleRegex.FindAllStringSubmatch(RowData, -1)
					CharacterInfoData.Title = subma1t[0][1]
					CharacterInfoData.UnlockedTitles = TibiaDataStringToInteger(subma1t[0][2])
				case "Vocation:":
					CharacterInfoData.Vocation = RowData
				case "Level:":
					CharacterInfoData.Level = TibiaDataStringToInteger(RowData)
				case "nobr", "Achievement Points:":
					CharacterInfoData.AchievementPoints = TibiaDataStringToInteger(RowData)
				case "World:":
					CharacterInfoData.World = RowData
				case "Former World:":
					CharacterInfoData.FormerWorlds = strings.Split(RowData, ", ")
				case "Residence:":
					CharacterInfoData.Residence = RowData
				case "Account Status:":
					CharacterInfoData.AccountStatus = RowData
				case "Married To:":
					AnchorQuery := s.Find("a")
					CharacterInfoData.MarriedTo = AnchorQuery.Nodes[0].FirstChild.Data
				case "House:":
					AnchorQuery := s.Find("a")
					HouseName := AnchorQuery.Nodes[0].FirstChild.Data
					HouseHref := AnchorQuery.Nodes[0].Attr[0].Val
					//substring from houseid= to &character in the href for the house
					HouseId := HouseHref[strings.Index(HouseHref, "houseid")+8 : strings.Index(HouseHref, "&character")]
					HouseRawData := RowNameQuery.Nodes[0].NextSibling.LastChild.Data
					HouseTown := HouseRawData[strings.Index(HouseRawData, "(")+1 : strings.Index(HouseRawData, ")")]
					HousePaidUntil := HouseRawData[strings.Index(HouseRawData, "is paid until ")+14:]

					CharacterInfoData.Houses = append(CharacterInfoData.Houses, Houses{
						Name:    HouseName,
						Town:    HouseTown,
						Paid:    TibiaDataDate(HousePaidUntil),
						HouseID: TibiaDataStringToInteger(HouseId),
					})
				case "Guild Membership:":
					CharacterInfoData.Guild.Rank = strings.TrimSuffix(RowData, " of the ")

					//TODO: I don't understand why the unicode nbsp is there...
					CharacterInfoData.Guild.GuildName = TibiaDataSanitizeStrings(RowNameQuery.Nodes[0].NextSibling.LastChild.LastChild.Data)
				case "Last Login:":
					if RowData != "never logged in" {
						CharacterInfoData.LastLogin = TibiaDataDatetime(RowData)
					}
				case "Comment:":
					node := RowNameQuery.Nodes[0].NextSibling.FirstChild

					stringBuilder := strings.Builder{}
					for node != nil {
						if node.DataAtom == Br {
							//It appears we can ignore br because either the encoding or goquery adds an \n for us
							//stringBuilder.WriteString("\n")
						} else {
							stringBuilder.WriteString(node.Data)
						}

						node = node.NextSibling
					}

					CharacterInfoData.Comment = stringBuilder.String()
				case "Loyalty Title:":
					if RowData != "(no title)" {
						AccountInformationData.LoyaltyTitle = RowData
					}
				case "Created:":
					AccountInformationData.Created = TibiaDataDatetime(RowData)
				case "Position:":
					TmpPosition := strings.Split(RowData, "<")
					if SectionName == "Character Information" {
						CharacterInfoData.Position = strings.TrimSpace(TmpPosition[0])
					} else if SectionName == "Account Information" {
						AccountInformationData.Position = strings.TrimSpace(TmpPosition[0])
					}

				default:
					log.Println("LEFT OVER: `" + RowName + "` = `" + RowData + "`")
				}
			})
		case "Account Badges":
			// Running query over each tr in list
			CharacterDivQuery.Find(".TableContentContainer tr td span[style]").EachWithBreak(func(index int, s *goquery.Selection) bool {
				// Storing HTML into CharacterListHTML
				CharacterListHTML, err := s.Html()
				if err != nil {
					insideError = fmt.Errorf("[error] TibiaCharactersCharacterImpl failed at s.Html() inside Account Badges, err: %s", err)
					return false
				}

				// Removing line breaks
				CharacterListHTML = TibiaDataHTMLRemoveLinebreaks(CharacterListHTML)

				// prevent failure of regex that parses account badges
				if CharacterListHTML != "There are no account badges set to be displayed for this character." {
					subma1 := accountBadgesRegex.FindAllStringSubmatch(CharacterListHTML, -1)

					AccountBadgesData = append(AccountBadgesData, AccountBadges{
						Name:        subma1[0][1],
						IconURL:     subma1[0][3],
						Description: subma1[0][2],
					})
				}

				return true
			})
		case "Account Achievements":
			// Running query over each tr in list
			CharacterDivQuery.Find(localDivQueryString).EachWithBreak(func(index int, s *goquery.Selection) bool {
				// Storing HTML into CharacterListHTML
				CharacterListHTML, err := s.Html()
				if err != nil {
					insideError = fmt.Errorf("[error] TibiaCharactersCharacterImpl failed at s.Html() inside Account Achievements, err: %s", err)
					return false
				}

				// Removing line breaks
				CharacterListHTML = TibiaDataHTMLRemoveLinebreaks(CharacterListHTML)

				subma1a := accountAchievementsRegex.FindAllStringSubmatch(CharacterListHTML, -1)
				if len(subma1a) > 0 {
					// fixing encoding for achievement name
					subma1a[0][2] = TibiaDataSanitizeEscapedString(subma1a[0][2])

					// get the name of the achievement (and ignore the secret image on the right)
					Name := strings.Split(subma1a[0][2], "<img")

					AchievementsData = append(AchievementsData, Achievements{
						Name:   Name[0],
						Grade:  strings.Count(subma1a[0][1], "achievement-grade-symbol"),
						Secret: strings.Contains(subma1a[0][2], "achievement-secret-symbol"),
					})
				}

				return true
			})
		case "Character Deaths":
			// Running query over each tr in list
			CharacterDivQuery.Find(localDivQueryString).EachWithBreak(func(index int, s *goquery.Selection) bool {
				// Storing HTML into CharacterListHTML
				CharacterListHTML, err := s.Html()
				if err != nil {
					insideError = fmt.Errorf("[error] TibiaCharactersCharacterImpl failed at s.Html() inside Character Deaths, err: %s", err)
					return false
				}

				// Removing line breaks
				CharacterListHTML = TibiaDataHTMLRemoveLinebreaks(CharacterListHTML)
				CharacterListHTML = strings.ReplaceAll(CharacterListHTML, ".<br/>Assisted by", ". Assisted by")

				// Regex to get data for deaths
				subma1 := deathRegex.FindAllStringSubmatch(CharacterListHTML, -1)

				if len(subma1) > 0 {
					// defining responses
					DeathKillers := []Killers{}
					DeathAssists := []Killers{}

					// store for reply later on.. and sanitizing string
					ReasonString := TibiaDataSanitizeStrings(RemoveHtmlTag(subma1[0][2] + " at Level " + subma1[0][3] + " by " + subma1[0][4] + "."))

					// if kill is with assist..
					if strings.Contains(subma1[0][4], ". Assisted by ") {
						TmpListOfDeath := strings.Split(subma1[0][4], ". Assisted by ")
						subma1[0][4] = TmpListOfDeath[0]
						TmpAssist := TmpListOfDeath[1]

						// get a list of killers
						ListOfAssists := strings.Split(TmpAssist, ", ")

						// extract if "and" is in last ss1
						ListOfAssistsTmp := strings.Split(ListOfAssists[len(ListOfAssists)-1], " and ")

						// if there is an "and", then we split it..
						if len(ListOfAssistsTmp) > 1 {
							ListOfAssists[len(ListOfAssists)-1] = ListOfAssistsTmp[0]
							ListOfAssists = append(ListOfAssists, ListOfAssistsTmp[1])
						}

						// loop through all killers and append to result
						for i := range ListOfAssists {
							name, isPlayer, isTraded, theSummon := TibiaDataParseKiller(ListOfAssists[i])
							DeathAssists = append(DeathAssists, Killers{
								Name:   name,
								Player: isPlayer,
								Traded: isTraded,
								Summon: theSummon,
							})
						}
					}

					// get a list of killers
					ListOfKillers := strings.Split(subma1[0][4], ", ")

					// extract if "and" is in last ss1
					ListOfKillersTmp := strings.Split(ListOfKillers[len(ListOfKillers)-1], " and ")

					// if there is an "and", then we split it..
					if len(ListOfKillersTmp) > 1 {
						ListOfKillers[len(ListOfKillers)-1] = ListOfKillersTmp[0]
						ListOfKillers = append(ListOfKillers, ListOfKillersTmp[1])
					}

					// loop through all killers and append to result
					for i := range ListOfKillers {
						name, isPlayer, isTraded, theSummon := TibiaDataParseKiller(ListOfKillers[i])
						DeathKillers = append(DeathKillers, Killers{
							Name:   name,
							Player: isPlayer,
							Traded: isTraded,
							Summon: theSummon,
						})
					}

					// append deadentry to death list
					DeathsData = append(DeathsData, Deaths{
						Time:    TibiaDataDatetime(subma1[0][1]),
						Level:   TibiaDataStringToInteger(subma1[0][3]),
						Killers: DeathKillers,
						Assists: DeathAssists,
						Reason:  ReasonString,
					})
				}

				return true
			})
		case "Characters":
			// Running query over each tr in character list
			CharacterDivQuery.Find(localDivQueryString).EachWithBreak(func(index int, s *goquery.Selection) bool {
				// Storing HTML into CharacterListHTML
				CharacterListHTML, err := s.Html()
				if err != nil {
					insideError = fmt.Errorf("[error] TibiaCharactersCharacterImpl failed at s.Html() inside Characters, err: %s", err)
					return false
				}

				// Removing line breaks
				CharacterListHTML = TibiaDataHTMLRemoveLinebreaks(CharacterListHTML)

				subma1 := characterInfoRegex.FindAllStringSubmatch(CharacterListHTML, -1)

				if len(subma1) > 0 {
					TmpCharacterName := subma1[0][1]

					var TmpTraded bool
					if strings.Contains(TmpCharacterName, localTradedString) {
						TmpTraded = true
						TmpCharacterName = strings.ReplaceAll(TmpCharacterName, localTradedString, "")
					}

					// If this character is the main character of the account
					TmpMain := false
					if strings.Contains(TmpCharacterName, "Main Character") {
						TmpMain = true
						Tmp := strings.Split(TmpCharacterName, "<")
						TmpCharacterName = strings.TrimSpace(Tmp[0])
					}

					// If this character is online or offline
					TmpStatus := "offline"
					if strings.Contains(subma1[0][3], "<b class=\"green\">online</b>") {
						TmpStatus = "online"
					}

					// Is this character is deleted
					TmpDeleted := false
					if strings.Contains(subma1[0][3], "deleted") {
						TmpDeleted = true
					}

					// Is this character having a special position
					TmpPosition := ""
					if strings.Contains(subma1[0][3], "CipSoft Member") {
						TmpPosition = "CipSoft Member"
					}

					// Create the character and append it to the other characters list
					OtherCharactersData = append(OtherCharactersData, OtherCharacters{
						Name:     TibiaDataSanitizeStrings(TmpCharacterName),
						World:    subma1[0][2],
						Status:   TmpStatus,
						Deleted:  TmpDeleted,
						Main:     TmpMain,
						Traded:   TmpTraded,
						Position: TmpPosition,
					})
				}

				return true
			})
		}

		return true
	})

	// Build the character data
	charData := Character{
		CharacterInfoData,
		AccountBadgesData,
		AchievementsData,
		DeathsData,
		AccountInformationData,
		OtherCharactersData,
	}

	// Search for errors
	switch {
	case characterNotFound:
		return nil, validation.ErrorCharacterNotFound
	case insideError != nil:
		return nil, insideError
	case reflect.DeepEqual(charData, Character{}):
		// There are some rare cases where a character name would
		// bug out tibia.com (tíbia, for example) and then we would't
		// receive the character not found error, for these edge cases
		// we check if the char structure is empty, if it is, it means
		// the character has not been found
		//
		// Validating those names would also be a pain because of old
		// tibian names such as Kolskägg, which for whatever reason is valid
		return nil, validation.ErrorCharacterNotFound
	}

	//
	// Build the data-blob
	return &CharacterResponse{
		charData,
		Information{
			APIDetails: TibiaDataAPIDetails,
			Timestamp:  TibiaDataDatetime(""),
			Status: Status{
				HTTPCode: http.StatusOK,
			},
		},
	}, nil
}

// TibiaDataParseKiller func - insert a html string and get the killers back
func TibiaDataParseKiller(data string) (string, bool, bool, string) {
	var (
		// local strings used in this function
		localTradedString = " (traded)"

		isPlayer, isTraded bool
		theSummon          string
	)

	// check if killer is a traded player
	if strings.Contains(data, localTradedString) {
		isPlayer = true
		isTraded = true
		data = strings.ReplaceAll(data, localTradedString, "")
	}

	// check if killer is a player
	if strings.Contains(data, "https://www.tibia.com") {
		isPlayer = true
		data = RemoveHtmlTag(data)
	}

	// get summon information
	if strings.HasPrefix(data, "a ") || strings.HasPrefix(data, "an ") {
		if containsCreaturesWithOf(data) {
			// this is not a summon, since it is a creature with a of in the middle
		} else {
			rs := summonRegex.FindAllStringSubmatch(data, -1)
			if len(rs) >= 1 {
				theSummon = rs[0][1]
				data = rs[0][2]
			}
		}
	}

	// sanitizing string
	data = TibiaDataSanitizeStrings(data)

	return data, isPlayer, isTraded, theSummon
}

// containsCreaturesWithOf checks if creature is present in special creatures list
func containsCreaturesWithOf(str string) bool {
	// this list should be based on the https://assets.tibiadata.com/data.json creatures name and plural_name field (currently only singular version)
	creaturesWithOf := []string{
		"acolyte of darkness",
		"acolyte of the cult",
		"adept of the cult",
		"ancient spawn of morgathla",
		"aspect of power",
		"baby pet of chayenne",
		"bane of light",
		"bloom of doom",
		"bride of night",
		"cloak of terror",
		"energuardian of tales",
		"enlightened of the cult",
		"eruption of destruction",
		"essence of darkness",
		"essence of malice",
		"eye of the seven",
		"flame of omrafir",
		"fury of the emperor",
		"ghastly pet of chayenne",
		"ghost of a planegazer",
		"greater splinter of madness",
		"groupie of skyrr",
		"guardian of tales",
		"gust of wind",
		"hand of cursed fate",
		"harbinger of darkness",
		"herald of gloom",
		"izcandar champion of summer",
		"izcandar champion of winter",
		"lesser splinter of madness",
		"lord of the elements",
		"lost ghost of a planegazer",
		"memory of a banshee",
		"memory of a book",
		"memory of a carnisylvan",
		"memory of a dwarf",
		"memory of a faun",
		"memory of a frazzlemaw",
		"memory of a fungus",
		"memory of a golem",
		"memory of a hero",
		"memory of a hydra",
		"memory of a lizard",
		"memory of a mammoth",
		"memory of a manticore",
		"memory of a pirate",
		"memory of a scarab",
		"memory of a shaper",
		"memory of a vampire",
		"memory of a werelion",
		"memory of a wolf",
		"memory of a yalahari",
		"memory of an amazon",
		"memory of an elf",
		"memory of an insectoid",
		"memory of an ogre",
		"mighty splinter of madness",
		"minion of gaz'haragoth",
		"minion of versperoth",
		"monk of the order",
		"muse of penciljack",
		"nightmare of gaz'haragoth",
		"noble pet of chayenne",
		"novice of the cult",
		"pillar of death",
		"pillar of draining",
		"pillar of healing",
		"pillar of protection",
		"pillar of summoning",
		"priestess of the wild sun",
		"rage of mazoran",
		"reflection of mawhawk",
		"reflection of obujos",
		"reflection of a mage",
		"retainer of baeloc",
		"scorn of the emperor",
		"servant of tentugly",
		"shadow of boreth",
		"shadow of lersatio",
		"shadow of marziel",
		"shard of corruption",
		"shard of magnor",
		"sight of surrender",
		"son of verminor",
		"soul of dragonking zyrtarch",
		"spark of destruction",
		"spawn of despair",
		"spawn of devovorga",
		"spawn of havoc",
		"spawn of the schnitzel",
		"spawn of the welter",
		"sphere of wrath",
		"spirit of earth",
		"spirit of fertility",
		"spirit of fire",
		"spirit of light",
		"spirit of water",
		"spite of the emperor",
		"squire of nictros",
		"stolen knowledge of armor",
		"stolen knowledge of healing",
		"stolen knowledge of lifesteal",
		"stolen knowledge of spells",
		"stolen knowledge of summoning",
		"stolen tome of portals",
		"sword of vengeance",
		"symbol of fear",
		"symbol of hatred",
		"tentacle of the deep terror",
		"the book of death",
		"the book of secrets",
		"the cold of winter",
		"the corruptor of souls",
		"the count of the core",
		"the devourer of secrets",
		"the duke of the depths",
		"the heat of summer",
		"the lily of night",
		"the lord of the lice",
		"the scion of havoc",
		"the scourge of oblivion",
		"the source of corruption",
		"the voice of ruin",
		"tin lizzard of lyxoph",
		"undead pet of chayenne",
		"weak harbinger of darkness",
		"weak spawn of despair",
		"wildness of urmahlullu",
		"wisdom of urmahlullu",
		"wrath of the emperor",
		"zarcorix of yalahar",
	}

	// trim away "an " and "a "
	str = strings.TrimPrefix(strings.TrimPrefix(str, "an "), "a ")

	for _, v := range creaturesWithOf {
		if v == str {
			return true
		}
	}
	return false
}
