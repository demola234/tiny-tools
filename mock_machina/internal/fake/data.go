package fake

type locale struct {
	firstNames []string
	lastNames  []string
	cities     []string
	countries  []string
	companies  []string
	phone      func(r digits) string
}

type digits func(n int) int

var locales = map[string]locale{
	"en": {
		firstNames: []string{
			"Ada", "Alan", "Alice", "Amelia", "Ben", "Charlotte", "Chloe", "Daniel", "Ella", "Emily",
			"Ethan", "Grace", "Hannah", "Harry", "Isla", "Jack", "James", "Leo", "Lily", "Lucas",
			"Maya", "Mia", "Noah", "Oliver", "Olivia", "Owen", "Ruby", "Samuel", "Sophie", "Thomas",
		},
		lastNames: []string{
			"Anderson", "Baker", "Bennett", "Brooks", "Carter", "Clarke", "Collins", "Cooper", "Davies", "Edwards",
			"Evans", "Fisher", "Foster", "Graham", "Hall", "Harris", "Hughes", "Jenkins", "Kelly", "Lewis",
			"Mitchell", "Morgan", "Murphy", "Parker", "Reed", "Russell", "Spencer", "Turner", "Walker", "Wright",
		},
		cities: []string{
			"Austin", "Boston", "Bristol", "Chicago", "Denver", "Dublin", "Edinburgh", "Leeds", "Manchester", "Melbourne",
			"Portland", "Seattle", "Sydney", "Toronto", "Vancouver",
		},
		countries: []string{"Australia", "Canada", "Ireland", "New Zealand", "United Kingdom", "United States"},
		companies: []string{
			"Blue Harbor Labs", "Brightline Studio", "Copperleaf Goods", "Fairway Analytics", "Granite Peak Supply",
			"Harbourside Media", "Juniper Health", "Maple Row Bakery", "Northwind Traders", "Silver Pine Logistics",
		},
		phone: func(d digits) string {
			return "+1 " + pad(200+d(800), 3) + "-555-01" + pad(d(100), 2)
		},
	},
	"en_NG": {
		firstNames: []string{
			"Adaeze", "Adebayo", "Aisha", "Amaka", "Bolaji", "Chidi", "Chiamaka", "Chinedu", "Damilola", "Emeka",
			"Fatima", "Folake", "Ibrahim", "Ifeoma", "Kelechi", "Kemi", "Musa", "Ngozi", "Nnamdi", "Obinna",
			"Oluwaseun", "Segun", "Tobi", "Tunde", "Uche", "Yemi", "Yusuf", "Zainab", "Funmilayo", "Babajide",
		},
		lastNames: []string{
			"Abubakar", "Adebayo", "Adeyemi", "Akande", "Bello", "Chukwu", "Danjuma", "Eze", "Ibekwe", "Lawal",
			"Mohammed", "Nwachukwu", "Nwosu", "Obi", "Odukoya", "Ogunleye", "Okafor", "Okeke", "Okonkwo", "Olaniyan",
			"Onyeka", "Oyelaran", "Sanni", "Suleiman", "Uzor", "Yakubu", "Usman", "Balogun", "Ojo", "Ikenna",
		},
		cities: []string{
			"Abeokuta", "Abuja", "Benin City", "Calabar", "Enugu", "Ibadan", "Ilorin", "Jos", "Kaduna", "Kano",
			"Lagos", "Onitsha", "Owerri", "Port Harcourt", "Uyo",
		},
		countries: []string{"Nigeria"},
		companies: []string{
			"Ikeja Fresh Foods", "Lekki Lane Studio", "Niger Delta Freight", "Oyo Craft Works", "Sahel Logistics",
			"Savannah Agro", "Yaba Code House", "Zuma Rock Media", "Calabar Spice Co", "Jos Plateau Dairies",
		},
		phone: func(d digits) string {
			return "+234 " + string(rune('7'+d(3))) + "0" + pad(d(10), 1) + " " + pad(d(1000), 3) + " " + pad(d(10000), 4)
		},
	},
}

var words = []string{
	"apple", "bright", "canvas", "delta", "ember", "forest", "garden", "harbor", "island", "jungle",
	"kettle", "lantern", "meadow", "nectar", "orbit", "pepper", "quiet", "river", "silver", "timber",
	"umbrella", "valley", "willow", "yellow", "zephyr", "basket", "copper", "dawn", "feather", "granite",
	"honey", "ivory", "jasmine", "lemon", "marble", "north", "ocean", "pebble", "rocket", "summer",
}
