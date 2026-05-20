// Package profile carries the Profile resource's wire types. Profile
// API methods still live on github.com/dmalch/go-geni's root Client
// during the pre-1.0 reshape and migrate into this package in a later
// PR; this PR lifts only the types so other resource sub-packages can
// reference them without circular imports.
package profile

// DetailsString carries localised free-text profile fields (currently
// just AboutMe). Used by [Profile.DetailStrings] and [Request.DetailStrings].
type DetailsString struct {
	// AboutMe is the profile's about me section
	AboutMe *string `json:"about_me"`
}

// Request is the JSON-encoded body sent to profile-create / -update
// endpoints. Some scalar fields (Title, Occupation, Suffix) are
// deliberately serialised without omitempty so that the empty string
// "" can clear a previously-set value — the Geni API treats "" as a
// clear sentinel for flat scalars but ignores omitted keys.
type Request struct {
	// DisplayName is the profile's display name
	DisplayName *string `json:"display_name,omitempty"`
	// Nicknames is the profile's nicknames
	Nicknames []string `json:"nicknames,omitempty"`
	// Gender is the profile's gender
	Gender *string `json:"gender,omitempty"`
	// Names is the name info
	Names map[string]NameElement `json:"names,omitempty"`
	// Birth is the birth event info
	Birth *EventElement `json:"birth,omitempty"`
	// Baptism is the baptism event info
	Baptism *EventElement `json:"baptism,omitempty"`
	// Death is the death event info
	Death *EventElement `json:"death,omitempty"`
	// CauseOfDeath is the cause of death
	CauseOfDeath *string `json:"cause_of_death"`
	// Burial is the burial event info
	Burial *EventElement `json:"burial,omitempty"`
	// IsAlive is a boolean that indicates whether the profile is living
	IsAlive bool `json:"is_alive"`
	// Title is the profile's name title. Sent without omitempty so that an
	// empty string clears any previously-set value (Geni accepts "" as a
	// clear sentinel for flat scalar fields).
	Title string `json:"title"`
	// CurrentResidence is the profile's current residence
	CurrentResidence *LocationElement `json:"current_residence"`
	// AboutMe is the profile's about me section
	AboutMe *string `json:"about_me"`
	// DetailStrings are nested maps of locales to details fields (e.g.
	// about me) to values. Tagged with omitempty: the Geni API crashes
	// (500, Ruby NoMethodError on nil) when a request body contains
	// "detail_strings": null. Callers who want to clear all details
	// must send an explicit empty map.
	DetailStrings map[string]DetailsString `json:"detail_strings,omitempty"`
	// Occupation is the profile's occupation. Sent without omitempty — see Title.
	Occupation string `json:"occupation"`
	// Suffix is the profile's suffix. Sent without omitempty — see Title.
	Suffix string `json:"suffix"`
	// Public is a boolean that indicates whether the profile is public
	Public bool `json:"public"`
	// Locked is a boolean that indicates whether the profile is locked down by a curator
	Locked bool `json:"locked"`
	// MergeNote is the note explaining the profile's merge status
	MergeNote []string `json:"merge_note,omitempty"`
}

// BulkResponse is the paginated envelope returned by profile-bulk
// endpoints (GetProfiles, SearchProfiles, listings on profile-scoped
// sub-resources, …).
type BulkResponse struct {
	Results    []Profile `json:"results,omitempty"`
	Page       int       `json:"page,omitempty"`
	TotalCount int       `json:"total_count,omitempty"`
	// NextPage / PrevPage are populated by paginated endpoints
	// (search, managed-profiles, …) when more pages are available.
	NextPage string `json:"next_page,omitempty"`
	PrevPage string `json:"prev_page,omitempty"`
}

// Profile is Geni's Profile resource — a single person in the family
// tree. Returned by every profile-fetching endpoint.
type Profile struct {
	// ID is the profile's node id
	ID string `json:"id,omitempty"`
	// Guid is the profile's globally unique identifier
	Guid string `json:"guid,omitempty"`
	// FirstName is the profile's first name
	FirstName *string `json:"first_name,omitempty"`
	// LastName is the profile's last name
	LastName *string `json:"last_name,omitempty"`
	// MiddleName is the profile's middle name
	MiddleName *string `json:"middle_name,omitempty"`
	// MaidenName is the profile's maiden name
	MaidenName *string `json:"maiden_name,omitempty"`
	// DisplayName is the profile's display name
	DisplayName *string `json:"display_name,omitempty"`
	// Nicknames is the profile's nicknames
	Nicknames []string `json:"nicknames,omitempty"`
	// Gender is the profile's gender
	Gender *string `json:"gender,omitempty"`
	// Names is the name info
	Names map[string]NameElement `json:"names,omitempty"`
	// Birth is the birth event info
	Birth *EventElement `json:"birth,omitempty"`
	// Baptism is the baptism event info
	Baptism *EventElement `json:"baptism,omitempty"`
	// Death is the death event info
	Death *EventElement `json:"death,omitempty"`
	// CauseOfDeath is the cause of death
	CauseOfDeath *string `json:"cause_of_death,omitempty"`
	// Burial is the burial event info
	Burial *EventElement `json:"burial,omitempty"`
	// Events is the events associated with this profile
	Events []EventElement `json:"events,omitempty"`
	// IsAlive is a boolean that indicates whether the profile is living
	IsAlive bool `json:"is_alive"`
	// Title is the profile's name title
	Title string `json:"title,omitempty"`
	// CurrentResidence is the profile's current residence
	CurrentResidence *LocationElement `json:"current_residence"`
	// AboutMe is the profile's about me section
	AboutMe *string `json:"about_me,omitempty"`
	// DetailStrings are nested maps of locales to details fields (e.g. about me) to values
	DetailStrings map[string]DetailsString `json:"detail_strings,omitempty"`
	// Occupation is the profile's occupation
	Occupation string `json:"occupation,omitempty"`
	// Suffix is the profile's suffix
	Suffix string `json:"suffix,omitempty"`
	// Public is a boolean that indicates whether the profile is public
	Public bool `json:"public"`
	// Locked is a boolean that indicates whether the profile is locked down by a curator
	Locked bool `json:"locked"`
	// Language is the profile's language
	Language string `json:"language,omitempty"`
	// ProfileUrl is the URL to access profile in a browser
	ProfileUrl string `json:"profile_url,omitempty"`
	// MergePending is a boolean that indicates whether the profile has a pending merge
	MergePending bool `json:"merge_pending,omitempty"`
	// MergedInto is the ID of the profile this profile is currently merged into
	MergedInto string `json:"merged_into,omitempty"`
	// MergeNote is the note explaining the profile's merge status
	MergeNote []string `json:"merge_note,omitempty"`
	// Url is the URL to access profile through the API
	Url string `json:"url,omitempty"`
	// Unions is the URLs to unions
	Unions []string `json:"unions,omitempty"`
	// Projects are the IDs of projects this profile is a member of
	Projects []string `json:"project_ids,omitempty"`
	// Deleted is a boolean that indicates whether the profile is deleted
	Deleted bool `json:"deleted"`
	// UpdatedAt is the timestamp of when the profile was last updated
	UpdatedAt string `json:"updated_at,omitempty"`
	// CreatedAt is the timestamp of when the profile was created
	CreatedAt string `json:"created_at,omitempty"`
}

// NameElement is the response for a name.
type NameElement struct {
	// FirstName is the profile's first name
	FirstName *string `json:"first_name"`
	// LastName is the profile's last name
	LastName *string `json:"last_name"`
	// MiddleName is the profile's middle name
	MiddleName *string `json:"middle_name"`
	// MaidenName is the profile's maiden name
	MaidenName *string `json:"maiden_name"`
	// DisplayName is the profile's display name
	DisplayName *string `json:"display_name"`
	// Nicknames is the profile's comma-separated list of nicknames
	Nicknames *string `json:"nicknames"`
}

// EventElement is the response for an event.
type EventElement struct {
	Date        *DateElement     `json:"date"`
	Description *string          `json:"description,omitempty"`
	Location    *LocationElement `json:"location"`
	Name        string           `json:"name,omitempty"`
}

// DateElement is the response for a date.
type DateElement struct {
	// Circa is a boolean that indicates whether the date is approximate
	Circa *bool `json:"circa"`
	// Day is the day of the month
	Day *int32 `json:"day"`
	// Month is the month of the year
	Month *int32 `json:"month"`
	// Year is the year
	Year *int32 `json:"year"`
	// EndCirca is a boolean that indicates whether the end date is approximate
	EndCirca *bool `json:"end_circa"`
	// EndDay is the end day of the month (only valid if range is between)
	EndDay *int32 `json:"end_day"`
	// EndMonth is the end month of the year (only valid if range is between)
	EndMonth *int32 `json:"end_month"`
	// EndYear is the end year (only valid if range is between)
	EndYear *int32 `json:"end_year"`
	// Range is the range (before, after, or between)
	Range *string `json:"range"`
}

// LocationElement is the response for a location.
type LocationElement struct {
	// City is the city name
	City *string `json:"city"`
	// Country is the country name
	Country *string `json:"country"`
	// County is the county name
	County *string `json:"county"`
	// Latitude is the latitude
	Latitude *float64 `json:"latitude,omitempty"`
	// Longitude is the longitude
	Longitude *float64 `json:"longitude,omitempty"`
	// PlaceName is the place name
	PlaceName *string `json:"place_name"`
	// State is the state name
	State *string `json:"state"`
	// StreetAddress1 is the street address line 1
	StreetAddress1 *string `json:"street_address1"`
	// StreetAddress2 is the street address line 2
	StreetAddress2 *string `json:"street_address2"`
	// StreetAddress3 is the street address line 3
	StreetAddress3 *string `json:"street_address3"`
}
