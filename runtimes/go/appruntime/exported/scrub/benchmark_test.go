package scrub

import (
	"encoding/json"
	"testing"
)

var benchmarkTotal int64 // prevent compiler from optimizing away the benchmark

func BenchmarkJSONIndices(b *testing.B) {
	for i := 0; i < b.N; i++ {
		bounds := JSONIndices(benchJSON, benchPaths)
		benchmarkTotal += int64(len(bounds))
	}
	b.ReportMetric(float64(len(benchJSON)*b.N)/float64(b.Elapsed().Seconds())/(1024.0*1024.0), "MiB/s")
}

func BenchmarkStdlibUnmarshal(b *testing.B) {
	for i := 0; i < b.N; i++ {
		data := new(benchData)
		if err := json.Unmarshal(benchJSON, &data); err != nil {
			b.Fatal(err)
		}
		benchmarkTotal += int64(len(data.Data))
	}
	b.ReportMetric(float64(len(benchJSON)*b.N)/float64(b.Elapsed().Seconds())/(1024.0*1024.0), "MiB/s")
}

var benchPaths = []Path{
	// data[*].friends[*].name
	{
		{Kind: ObjectField, FieldName: "data", CaseSensitive: true},
		{Kind: ObjectField, FieldName: "friends", CaseSensitive: true},
		{Kind: ObjectField, FieldName: "name", CaseSensitive: true},
	},
	// data[*].name
	{
		{Kind: ObjectField, FieldName: "data", CaseSensitive: true},
		{Kind: ObjectField, FieldName: "name", CaseSensitive: true},
	},
	// data[*].address
	{
		{Kind: ObjectField, FieldName: "data", CaseSensitive: true},
		{Kind: ObjectField, FieldName: "address", CaseSensitive: true},
	},
	// data[*].email
	{
		{Kind: ObjectField, FieldName: "data", CaseSensitive: true},
		{Kind: ObjectField, FieldName: "email", CaseSensitive: true},
	},
}

// benchJSON is randomly generated sample data.
var benchJSON = []byte(`
{"data": [
  {
    "_id": "6481d7ace923338d11cdecaf",
    "index": 0,
    "guid": "04d97d05-b475-4465-b15d-de7159794dac",
    "isActive": true,
    "balance": "$2,218.53",
    "picture": "http://placehold.it/32x32",
    "age": 31,
    "eyeColor": "green",
    "name": "Jo Pugh",
    "gender": "female",
    "company": "QUANTASIS",
    "email": "jopugh@quantasis.com",
    "phone": "+1 (934) 590-3966",
    "address": "955 Granite Street, Nutrioso, West Virginia, 3957",
    "about": "Culpa ut et in sit officia voluptate. Anim dolor esse qui pariatur nisi exercitation nostrud tempor labore sint. Est duis ut est nulla in tempor id velit minim deserunt ullamco laborum. Qui dolor nostrud anim deserunt nostrud anim adipisicing nostrud aliqua. Adipisicing occaecat adipisicing duis et labore sunt id ea et magna adipisicing labore. Consectetur tempor mollit mollit duis nostrud officia in elit. Dolor dolore deserunt fugiat labore aliqua fugiat irure sunt ipsum veniam.\r\n",
    "registered": "2022-07-01T04:27:58 -02:00",
    "latitude": -57.797481,
    "longitude": 84.309916,
    "tags": [
      "ad",
      "eiusmod",
      "ex",
      "dolor",
      "sunt",
      "excepteur",
      "duis"
    ],
    "friends": [
      {
        "id": 0,
        "name": "Bray Hogan"
      },
      {
        "id": 1,
        "name": "Hope Conley"
      },
      {
        "id": 2,
        "name": "Rollins Combs"
      }
    ],
    "greeting": "Hello, Jo Pugh! You have 7 unread messages.",
    "favoriteFruit": "strawberry"
  },
  {
    "_id": "6481d7ac6b1179b1585fca2f",
    "index": 1,
    "guid": "56da4017-435e-4d4a-a9b3-6ec8ada947f6",
    "isActive": true,
    "balance": "$1,492.03",
    "picture": "http://placehold.it/32x32",
    "age": 21,
    "eyeColor": "blue",
    "name": "Mills Nicholson",
    "gender": "male",
    "company": "BOILICON",
    "email": "millsnicholson@boilicon.com",
    "phone": "+1 (846) 499-2163",
    "address": "846 Albemarle Road, Celeryville, Florida, 3173",
    "about": "Et fugiat adipisicing sit eu. Irure enim occaecat sunt duis dolor. Qui mollit velit aute excepteur aliqua qui incididunt adipisicing tempor enim.\r\n",
    "registered": "2019-11-17T02:23:12 -01:00",
    "latitude": 39.918662,
    "longitude": 101.881282,
    "tags": [
      "consequat",
      "laboris",
      "minim",
      "id",
      "laborum",
      "labore",
      "cillum"
    ],
    "friends": [
      {
        "id": 0,
        "name": "Cleo Juarez"
      },
      {
        "id": 1,
        "name": "Aline Rowe"
      },
      {
        "id": 2,
        "name": "Ester Oconnor"
      }
    ],
    "greeting": "Hello, Mills Nicholson! You have 5 unread messages.",
    "favoriteFruit": "strawberry"
  },
  {
    "_id": "6481d7ac88ee534c9f5bfff0",
    "index": 2,
    "guid": "b46ff220-b872-4d1a-b095-2a16d30b4fbd",
    "isActive": false,
    "balance": "$1,673.74",
    "picture": "http://placehold.it/32x32",
    "age": 34,
    "eyeColor": "blue",
    "name": "Eunice Gould",
    "gender": "female",
    "company": "AVIT",
    "email": "eunicegould@avit.com",
    "phone": "+1 (962) 427-2923",
    "address": "477 Hinsdale Street, Edneyville, Oregon, 9253",
    "about": "Et Lorem qui reprehenderit incididunt labore est occaecat cupidatat ullamco mollit. Amet cillum consectetur ex sunt sint anim adipisicing duis veniam do qui enim anim cillum. Irure eu elit minim et est. Fugiat officia incididunt elit ut fugiat in do mollit ullamco sint in ut consequat. Occaecat deserunt dolor ipsum veniam aliqua mollit sit culpa elit ex.\r\n",
    "registered": "2018-06-17T11:52:20 -02:00",
    "latitude": 50.048343,
    "longitude": -78.508861,
    "tags": [
      "amet",
      "ullamco",
      "voluptate",
      "voluptate",
      "dolor",
      "elit",
      "id"
    ],
    "friends": [
      {
        "id": 0,
        "name": "Ericka Pennington"
      },
      {
        "id": 1,
        "name": "Cain Pearson"
      },
      {
        "id": 2,
        "name": "Pacheco Walters"
      }
    ],
    "greeting": "Hello, Eunice Gould! You have 10 unread messages.",
    "favoriteFruit": "banana"
  },
  {
    "_id": "6481d7acce8494cef525f2bc",
    "index": 3,
    "guid": "1498919d-fae5-4ade-aa4a-7eb332d899e6",
    "isActive": false,
    "balance": "$1,592.80",
    "picture": "http://placehold.it/32x32",
    "age": 33,
    "eyeColor": "brown",
    "name": "Lindsay Howe",
    "gender": "male",
    "company": "EXTRAGEN",
    "email": "lindsayhowe@extragen.com",
    "phone": "+1 (940) 473-3478",
    "address": "759 Montieth Street, Dellview, Wyoming, 3283",
    "about": "Commodo minim sint non id aute minim exercitation. In qui nulla ut non reprehenderit adipisicing duis elit. Dolore non aliqua est veniam nisi id eiusmod aliqua non fugiat labore. Ullamco dolore culpa aliquip id tempor laboris. Officia adipisicing pariatur ullamco sunt veniam pariatur ullamco ullamco nostrud nisi qui mollit. Nisi velit ut commodo magna qui fugiat eu ut amet qui consequat excepteur.\r\n",
    "registered": "2014-11-27T02:20:32 -01:00",
    "latitude": -53.18764,
    "longitude": -164.847231,
    "tags": [
      "qui",
      "nulla",
      "quis",
      "exercitation",
      "non",
      "occaecat",
      "est"
    ],
    "friends": [
      {
        "id": 0,
        "name": "Stone Gallagher"
      },
      {
        "id": 1,
        "name": "Chavez Pope"
      },
      {
        "id": 2,
        "name": "Farmer Mueller"
      }
    ],
    "greeting": "Hello, Lindsay Howe! You have 4 unread messages.",
    "favoriteFruit": "apple"
  },
  {
    "_id": "6481d7ac14c51c8748cbab88",
    "index": 4,
    "guid": "f72fa31e-bd75-46ce-a9bf-53675f50f01c",
    "isActive": true,
    "balance": "$1,160.87",
    "picture": "http://placehold.it/32x32",
    "age": 37,
    "eyeColor": "brown",
    "name": "Minerva Sims",
    "gender": "female",
    "company": "NETPLAX",
    "email": "minervasims@netplax.com",
    "phone": "+1 (887) 477-2537",
    "address": "864 Ryder Avenue, Tedrow, Virginia, 4180",
    "about": "Magna id sit adipisicing eiusmod fugiat nulla voluptate sint culpa sit consectetur est sint. Culpa velit eu velit non Lorem irure adipisicing fugiat culpa aliquip Lorem eiusmod. Occaecat ad est commodo est irure veniam magna deserunt laboris commodo quis cupidatat occaecat. Consequat exercitation officia commodo id. Incididunt magna sunt mollit labore. Proident exercitation est enim mollit reprehenderit Lorem anim. Fugiat consectetur elit anim reprehenderit.\r\n",
    "registered": "2014-02-05T11:36:57 -01:00",
    "latitude": 5.818084,
    "longitude": 156.041539,
    "tags": [
      "do",
      "amet",
      "ex",
      "dolor",
      "cupidatat",
      "dolor",
      "tempor"
    ],
    "friends": [
      {
        "id": 0,
        "name": "Karen Dickerson"
      },
      {
        "id": 1,
        "name": "Frye Jones"
      },
      {
        "id": 2,
        "name": "Rosie Travis"
      }
    ],
    "greeting": "Hello, Minerva Sims! You have 5 unread messages.",
    "favoriteFruit": "apple"
  },
  {
    "_id": "6481d7ac80e906f930d8deea",
    "index": 5,
    "guid": "17c377e0-0ba6-46ea-8faa-6c0301ffb3d4",
    "isActive": true,
    "balance": "$3,838.92",
    "picture": "http://placehold.it/32x32",
    "age": 37,
    "eyeColor": "green",
    "name": "Fischer Avery",
    "gender": "male",
    "company": "POLARIUM",
    "email": "fischeravery@polarium.com",
    "phone": "+1 (876) 442-3098",
    "address": "339 Wyona Street, Wadsworth, Arizona, 5446",
    "about": "Laborum sint consectetur et ex officia. Aliqua veniam aliquip aliqua pariatur deserunt quis esse nisi non amet culpa. Sint nostrud magna sit amet mollit fugiat non adipisicing.\r\n",
    "registered": "2022-01-30T12:04:00 -01:00",
    "latitude": 33.64373,
    "longitude": 114.28595,
    "tags": [
      "ex",
      "ipsum",
      "adipisicing",
      "sint",
      "elit",
      "et",
      "ad"
    ],
    "friends": [
      {
        "id": 0,
        "name": "Gayle Walker"
      },
      {
        "id": 1,
        "name": "Sherman Tyler"
      },
      {
        "id": 2,
        "name": "Gladys Shaffer"
      }
    ],
    "greeting": "Hello, Fischer Avery! You have 2 unread messages.",
    "favoriteFruit": "banana"
  }
]}
`)

type benchData struct {
	Data []struct {
		Id         string   `json:"_id"`
		Index      int      `json:"index"`
		Guid       string   `json:"guid"`
		IsActive   bool     `json:"isActive"`
		Balance    string   `json:"balance"`
		Picture    string   `json:"picture"`
		Age        int      `json:"age"`
		EyeColor   string   `json:"eyeColor"`
		Name       string   `json:"name"`
		Gender     string   `json:"gender"`
		Company    string   `json:"company"`
		Email      string   `json:"email"`
		Phone      string   `json:"phone"`
		Address    string   `json:"address"`
		About      string   `json:"about"`
		Registered string   `json:"registered"`
		Latitude   float64  `json:"latitude"`
		Longitude  float64  `json:"longitude"`
		Tags       []string `json:"tags"`
		Friends    []struct {
			Id   int    `json:"id"`
			Name string `json:"name"`
		} `json:"friends"`
		Greeting      string `json:"greeting"`
		FavoriteFruit string `json:"favoriteFruit"`
	} `json:"data"`
}
