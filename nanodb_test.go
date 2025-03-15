package nanodb

import (
	"encoding/json"
	"math/rand/v2"
	"os"
	"os/exec"
	"reflect"
	"slices"
	"sync"
	"testing"
	"time"
)

func TestNanodb(t *testing.T) {
	db := New[string]()

	wg := &sync.WaitGroup{}
	for _, key := range testKeys {
		wg.Add(1)
		go func() {
			defer wg.Done()
			db.Add(key, key)
		}()
	}
	wg.Wait()

	for _, key := range testKeys {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if db.Get(key) != key {
				t.Errorf("Get(%q) != %q", key, db.Get(key))
			}
		}()
	}
	wg.Wait()

	for range 2 {
		for key, value := range db.Seq2() {
			if key != value {
				t.Errorf("Seq2(%q) != %q", key, value)
			}
		}
	}

	for _, key := range testKeys {
		wg.Add(1)
		go func() {
			defer wg.Done()
			db.Del(key)
		}()
	}
	wg.Wait()

	if db.Len() != 0 {
		t.Errorf("db.Len() != 0")
	}
}

type TestingUser struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}

func TestNanodbCache(t *testing.T) {
	if err := exec.Command("cp", "testdata/cache.json", "testdata/cache_test.json").Run(); err != nil {
		t.Fatal(err)
	}

	db, err := From[*TestingUser]("testdata/cache_test.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove("testdata/cache_test.json")

	wg := &sync.WaitGroup{}
	for i, key := range testKeys {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := db.Add(key, &TestingUser{100 + i, key}); err != nil {
				panic(err)
			}
		}()
	}
	wg.Wait()

	for _, key := range testKeys {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := db.Del(key); err != nil {
				panic(err)
			}
		}()
	}
	wg.Wait()

	original := map[string]*TestingUser{}
	originalData, _ := os.ReadFile("testdata/cache.json")
	if err := json.Unmarshal(originalData, &original); err != nil {
		t.Fatal(err)
	}
	afterTest := map[string]*TestingUser{}
	afterData, _ := os.ReadFile("testdata/cache_test.json")
	if err := json.Unmarshal(afterData, &afterTest); err != nil {
		t.Fatal(err)
	}

	if len(original) != len(afterTest) {
		t.Errorf("len(original) != len(afterTest)")
	}
	for key, value := range original {
		if !reflect.DeepEqual(afterTest[key], value) {
			t.Errorf("afterTest[%q] != %q (%q)", key, value, afterTest[key])
		}
	}

	for range 2 {
		for _, value := range db.Seq2() {
			if slices.Contains(testKeys, value.Name) {
				t.Fatal("unexpected value in cache")
			}
		}
	}
}

/*
goos: darwin
goarch: arm64
pkg: github.com/kittenbark/nanodb
cpu: Apple M2
BenchmarkDB_Sequential
BenchmarkDB_Sequential-8   	 4039136	       287.0 ns/op
*/
func BenchmarkDB_Sequential(b *testing.B) {
	db := New[string]().Timeout(time.Millisecond * 10)
	keysN := len(testKeys)
	for b.Loop() {
		switch rand.N(3) {
		case 0:
			db.Add(testKeys[rand.N(keysN)], testKeys[rand.N(keysN)])
		case 1:
			db.TryGet(testKeys[rand.N(keysN)])
		case 2:
			db.Del(testKeys[rand.N(keysN)])
		}
	}
}

/*
goos: darwin
goarch: arm64
pkg: github.com/kittenbark/nanodb
cpu: Apple M2
BenchmarkDB_Parallel100
BenchmarkDB_Parallel100-8   	   17952	     65909 ns/op	-- note: goes through 100 goroutines per iteration.
*/
func BenchmarkDB_Parallel100(b *testing.B) {
	db := New[string]().Timeout(time.Millisecond * 10)
	keysN := len(testKeys)
	for b.Loop() {
		wg := &sync.WaitGroup{}
		for range 100 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				switch rand.N(3) {
				case 0:
					db.Add(testKeys[rand.N(keysN)], testKeys[rand.N(keysN)])
				case 1:
					db.TryGet(testKeys[rand.N(keysN)])
				case 2:
					db.Del(testKeys[rand.N(keysN)])
				}
			}()
		}
		wg.Wait()
	}
}

/*
goos: darwin
goarch: arm64
pkg: github.com/kittenbark/nanodb
cpu: Apple M2
BenchmarkDB_Cache_Sequential
BenchmarkDB_Cache_Sequential-8   	     106	  11183335 ns/op	-- note: goes through 100 goroutines per iteration.
PASS
*/
func BenchmarkDB_Cache_Sequential(b *testing.B) {
	db, err := From[string]("testdata/benchmark.json")
	if err != nil {
		b.Fatal(err)
	}
	defer os.Remove("testdata/benchmark.json")

	keysN := len(testKeys)
	for b.Loop() {
		wg := &sync.WaitGroup{}
		for range 100 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				switch rand.N(3) {
				case 0:
					if err := db.Add(testKeys[rand.N(keysN)], testKeys[rand.N(keysN)]); err != nil {
						b.Fatal(err)
					}
				case 1:
					if _, _, err := db.TryGet(testKeys[rand.N(keysN)]); err != nil {
						b.Fatal(err)
					}
				case 2:
					if err := db.Del(testKeys[rand.N(keysN)]); err != nil {
						b.Fatal(err)
					}
				}
			}()
		}
		wg.Wait()
	}
}

func BenchmarkDB_Cache_Parallel(b *testing.B) {
	db, err := From[string]("testdata/benchmark.json")
	if err != nil {
		b.Fatal(err)
	}
	defer os.Remove("testdata/benchmark.json")

	keysN := len(testKeys)
	for b.Loop() {
		switch rand.N(3) {
		case 0:
			if err := db.Add(testKeys[rand.N(keysN)], testKeys[rand.N(keysN)]); err != nil {
				b.Fatal(err)
			}
		case 1:
			if _, _, err := db.TryGet(testKeys[rand.N(keysN)]); err != nil {
				b.Fatal(err)
			}
		case 2:
			if err := db.Del(testKeys[rand.N(keysN)]); err != nil {
				b.Fatal(err)
			}
		}
	}
}

var testKeys = []string{
	"green", "cyan", "blue", "red", "yellow", "purple", "orange", "pink",
	"brown", "black", "white", "gray", "magenta", "violet", "indigo",
	"teal", "turquoise", "navy", "maroon", "olive", "lime", "coral",
	"salmon", "gold", "silver", "beige", "khaki", "tan", "chocolate",
	"crimson", "lavender", "plum", "ivory", "azure", "mint", "forest",
	"burgundy", "coffee", "rust", "amber", "charcoal", "platinum", "ruby",
	"emerald", "sapphire", "amethyst", "topaz", "jade", "pearl", "onyx",
	"obsidian", "garnet", "aquamarine", "opal", "quartz", "bronze", "copper",
	"brass", "steel", "titanium", "cobalt", "nickel", "zinc", "aluminum",
	"mercury", "lead", "tin", "iron", "chrome", "tungsten", "mahogany",
	"oak", "pine", "cedar", "birch", "maple", "walnut", "cherry", "ash",
	"willow", "bamboo", "rosewood", "elm", "hickory", "apple", "banana",
	"orange", "grape", "strawberry", "blueberry", "raspberry", "blackberry",
	"lemon", "lime", "mango", "kiwi", "pineapple", "peach", "pear", "plum",
	"apricot", "cherry", "watermelon", "cantaloupe", "honeydew", "fig",
	"date", "coconut", "avocado", "tomato", "potato", "carrot", "celery",
	"spinach", "lettuce", "broccoli", "cauliflower", "corn", "pea", "bean",
	"lentil", "rice", "wheat", "barley", "oat", "rye", "quinoa", "bread",
	"pasta", "noodle", "pizza", "burger", "sandwich", "taco", "burrito",
	"sushi", "curry", "soup", "salad", "stew", "roast", "steak", "chicken",
	"turkey", "duck", "goose", "pork", "beef", "lamb", "fish", "shrimp",
	"crab", "lobster", "oyster", "mussel", "clam", "scallop", "squid",
	"octopus", "jellyfish", "starfish", "coral", "seaweed", "shell", "sand",
	"beach", "ocean", "sea", "lake", "river", "stream", "pond", "pool",
	"waterfall", "mountain", "hill", "valley", "canyon", "cliff", "cave",
	"desert", "forest", "jungle", "rainforest", "tundra", "prairie", "meadow",
	"field", "garden", "park", "yard", "lawn", "sidewalk", "street", "road",
	"highway", "path", "trail", "bridge", "tunnel", "building", "house",
	"apartment", "condo", "mansion", "cabin", "cottage", "castle", "palace",
	"temple", "church", "cathedral", "mosque", "synagogue", "skyscraper",
	"tower", "monument", "museum", "library", "school", "college", "university",
	"hospital", "clinic", "pharmacy", "store", "shop", "mall", "market",
	"restaurant", "cafe", "bar", "pub", "club", "theater", "cinema", "stadium",
	"arena", "park", "zoo", "aquarium", "circus", "carnival", "fair", "festival",
	"parade", "concert", "play", "opera", "ballet", "dance", "sing", "music",
	"art", "painting", "drawing", "sketch", "sculpture", "statue", "photograph",
	"film", "movie", "show", "book", "novel", "story", "poem", "letter", "essay",
	"report", "paper", "journal", "magazine", "newspaper", "comic", "manual",
	"guide", "map", "atlas", "globe", "compass", "calculator", "computer", "laptop",
	"tablet", "phone", "camera", "radio", "television", "clock", "watch", "calendar",
	"wallet", "purse", "bag", "backpack", "luggage", "umbrella", "hat", "cap",
	"crown", "helmet", "mask", "glasses", "sunglasses", "glove", "mitten", "scarf",
	"tie", "shirt", "blouse", "sweater", "jacket", "coat", "vest", "dress", "skirt",
	"pants", "jeans", "shorts", "sock", "shoe", "boot", "sandal", "slipper", "ring",
	"necklace", "bracelet", "earring", "pendant", "brooch", "comb", "brush", "soap",
	"shampoo", "towel", "toothbrush", "toothpaste", "floss", "razor", "mirror", "perfume",
	"cologne", "makeup", "lipstick", "nail", "hair", "beard", "mustache", "eye", "ear",
	"nose", "mouth", "lip", "tooth", "tongue", "throat", "neck", "shoulder", "arm", "elbow",
	"wrist", "hand", "finger", "thumb", "nail", "heart", "lung", "liver", "kidney", "stomach",
	"intestine", "brain", "spine", "bone", "muscle", "skin", "blood",
}
