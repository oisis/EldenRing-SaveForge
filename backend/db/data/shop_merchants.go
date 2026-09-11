package data

// ShopRowRange is an inclusive ShopLineupParam row-ID range owned by one merchant.
type ShopRowRange struct {
	First int32
	Last  int32
}

// ShopMerchant maps one browsable merchant NPC to the ShopLineupParam rows that
// hold its retail stock.
type ShopMerchant struct {
	Name string
	Rows []ShopRowRange
}

// ShopMerchants is the Super Merchant merchant → ShopLineupParam row mapping for
// Regulation 1.17 (PC).
//
// Provenance: row-ID blocks are not self-describing — ShopLineupParam carries no
// row names in the save's embedded regulation, and shop → NPC ownership lives in
// ESD/EMEVD, not in the PARAM. This table transcribes the attribution facts and
// normalization rules documented by the ER merchant-editor research project
// (docs/MERCHANTS.md and its generated merchant_catalog.json), including its
// known row corrections:
//
//   - 100250-100252 belong to Sorcerer Thops, not Preceptor Seluvis;
//   - the Raya Lucaria generic merchant block belongs to the Isolated Merchant;
//   - quest/scroll/prayerbook tiers of Corhyn, Miriel, Sellen, Seluvis and the
//     Twin Maiden Husks collapse into one merchant identity each;
//   - Regulation 1.17 retail additions are folded into their owning blocks
//     (100568, 100666-100669, 100709-100713, 101896).
//
// Deliberately excluded, and never browsable here:
//
//   - Enia — her stock is material-priced and release-flag gated;
//   - Dragon Communion and other non-NPC special exchanges;
//   - Alteration/Reversion armor-mechanic rows;
//   - debug, duplicate and unreachable rows.
//
// Rows whose mtrlId is not -1 are additionally rejected at read time, so a
// material-priced row inside a listed range stays read-only.
var ShopMerchants = []ShopMerchant{
	{Name: "Abandoned Merchant - Siofra River", Rows: []ShopRowRange{{100925, 100926}, {100928, 100928}, {100935, 100941}}},
	{Name: "Blackguard Big Boggart", Rows: []ShopRowRange{{100150, 100150}, {100155, 100155}, {100160, 100160}}},
	{Name: "Brother Corhyn", Rows: []ShopRowRange{{100350, 100378}}},
	{Name: "Count Ymir", Rows: []ShopRowRange{{102300, 102308}}},
	{Name: "D, Hunter of the Dead", Rows: []ShopRowRange{{100126, 100127}}},
	{Name: "Gatekeeper Gostoc", Rows: []ShopRowRange{{100000, 100000}, {100002, 100002}, {100004, 100004}, {100009, 100018}}},
	{Name: "Gowry", Rows: []ShopRowRange{{100175, 100177}, {100185, 100185}}},
	{Name: "Hermit Merchant - Ainsel River", Rows: []ShopRowRange{{100950, 100951}, {100953, 100953}, {100962, 100967}}},
	{Name: "Hermit Merchant - Leyndell", Rows: []ShopRowRange{{100725, 100743}}},
	{Name: "Hermit Merchant - Mountaintops of the Giants", Rows: []ShopRowRange{{100900, 100901}, {100903, 100903}, {100909, 100918}}},
	{Name: "Iji", Rows: []ShopRowRange{{100225, 100229}}},
	{Name: "Imprisoned Merchant - Mohgwyn", Rows: []ShopRowRange{{100976, 100977}, {100981, 100986}}},
	{Name: "Isolated Merchant - Academy of Raya Lucaria", Rows: []ShopRowRange{{100675, 100676}, {100678, 100678}, {100680, 100682}, {100686, 100696}}},
	{Name: "Isolated Merchant - Dragonbarrow", Rows: []ShopRowRange{{100875, 100878}, {100880, 100880}, {100882, 100893}}},
	{Name: "Isolated Merchant - Weeping Peninsula", Rows: []ShopRowRange{{100650, 100650}, {100652, 100652}, {100654, 100654}, {100656, 100669}}},
	{Name: "Knight Bernahl", Rows: []ShopRowRange{{100075, 100086}}},
	{Name: "Merchant Kale", Rows: []ShopRowRange{{100500, 100507}, {100510, 100511}, {100513, 100520}}},
	{Name: "Miriel, Pastor of Vows", Rows: []ShopRowRange{{100400, 100407}, {100425, 100426}, {100428, 100443}}},
	{Name: "Moore", Rows: []ShopRowRange{{102250, 102266}}},
	{Name: "Nomadic Merchant - Altus Plateau", Rows: []ShopRowRange{{100750, 100753}, {100761, 100761}, {100764, 100771}}},
	{Name: "Nomadic Merchant - Caelid (Aeonia Swamp)", Rows: []ShopRowRange{{100801, 100803}, {100813, 100817}}},
	{Name: "Nomadic Merchant - Coastal Cave", Rows: []ShopRowRange{{100575, 100576}, {100585, 100595}}},
	{Name: "Nomadic Merchant - East Limgrave", Rows: []ShopRowRange{{100550, 100554}, {100557, 100557}, {100560, 100568}}},
	{Name: "Nomadic Merchant - East Weeping Peninsula", Rows: []ShopRowRange{{100600, 100618}}},
	{Name: "Nomadic Merchant - Liurnia of the Lakes", Rows: []ShopRowRange{{100625, 100637}}},
	{Name: "Nomadic Merchant - Mt. Gelmir", Rows: []ShopRowRange{{100775, 100775}, {100777, 100777}, {100784, 100784}, {100789, 100798}}},
	{Name: "Nomadic Merchant - North Limgrave", Rows: []ShopRowRange{{100525, 100525}, {100539, 100547}}},
	{Name: "Nomadic Merchant - North Liurnia", Rows: []ShopRowRange{{100700, 100700}, {100704, 100706}, {100709, 100720}}},
	{Name: "Nomadic Merchant - South Caelid", Rows: []ShopRowRange{{100825, 100827}, {100837, 100845}}},
	{Name: "Patches", Rows: []ShopRowRange{{100100, 100105}, {100108, 100108}, {100111, 100122}}},
	{Name: "Pidia, Carian Servant", Rows: []ShopRowRange{{100325, 100326}, {100329, 100330}, {100334, 100340}}},
	{Name: "Preceptor Seluvis", Rows: []ShopRowRange{{100275, 100284}, {100300, 100302}, {100310, 100310}}},
	{Name: "Sorcerer Rogier", Rows: []ShopRowRange{{100200, 100202}}},
	{Name: "Sorcerer Thops", Rows: []ShopRowRange{{100250, 100252}}},
	{Name: "Sorceress Sellen", Rows: []ShopRowRange{{100050, 100062}}},
	{Name: "Thiollier", Rows: []ShopRowRange{{102270, 102282}}},
	{Name: "Twin Maiden Husks", Rows: []ShopRowRange{{101800, 101881}, {101885, 101896}}},
}
