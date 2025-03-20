package cmd

import (
	"fmt"
	"os"

	"cosmossdk.io/log"
	"cosmossdk.io/store/iavl"
	"cosmossdk.io/store/metrics"
	"cosmossdk.io/store/rootmulti"
	"cosmossdk.io/store/types"
	storetypes "cosmossdk.io/store/types"
	evidencetypes "cosmossdk.io/x/evidence/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	dbm "github.com/cosmos/cosmos-db"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	authzkeeper "github.com/cosmos/cosmos-sdk/x/authz/keeper"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	consensusparamtypes "github.com/cosmos/cosmos-sdk/x/consensus/types"
	crisistypes "github.com/cosmos/cosmos-sdk/x/crisis/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	packetforwardtypes "github.com/cosmos/ibc-apps/middleware/packet-forward-middleware/v8/packetforward/types"
	icqtypes "github.com/cosmos/ibc-apps/modules/async-icq/v8/types"
	icahosttypes "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/host/types"
	ibctransfertypes "github.com/cosmos/ibc-go/v8/modules/apps/transfer/types"
	ibchost "github.com/cosmos/ibc-go/v8/modules/core/exported"
	"github.com/spf13/cobra"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

func pruneStoreCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prune-store [path_to_home] [store_name]",
		Short: "prune data from a specific application store",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			home := args[0]
			storeName := args[1]
			return pruneSpecificStore(home, storeName)
		},
	}
	return cmd
}

func pruneSpecificStore(home, storeName string) error {
	dbDir := rootify(dataDir, home)

	o := opt.Options{
		DisableSeeksCompaction: true,
	}

	// Get BlockStore
	appDB, err := dbm.NewGoLevelDBWithOpts("application", dbDir, &o)
	if err != nil {
		return err
	}

	fmt.Printf("pruning store: %s\n", storeName)

	logger := log.NewLogger(os.Stderr)
	appStore := rootmulti.NewStore(appDB, logger, metrics.NewMetrics([][]string{}))

	// Mount all stores to ensure we can access the target store
	keys := storetypes.NewKVStoreKeys(
		authtypes.StoreKey, banktypes.StoreKey, authzkeeper.StoreKey, stakingtypes.StoreKey, distrtypes.StoreKey, slashingtypes.StoreKey, ibchost.StoreKey,
		icahosttypes.StoreKey,
		icqtypes.StoreKey,
		evidencetypes.StoreKey, minttypes.StoreKey, govtypes.StoreKey, ibctransfertypes.StoreKey,
		packetforwardtypes.StoreKey,
		paramstypes.StoreKey, consensusparamtypes.StoreKey, crisistypes.StoreKey, upgradetypes.StoreKey,
	)

	if app == "osmosis" {
		osmoKeys := storetypes.NewKVStoreKeys(
			"downtimedetector",
			"hooks-for-ibc",
			"lockup",
			"concentratedliquidity",
			"gamm",
			"cosmwasmpool",
			"poolmanager",
			"twap",
			"epochs",
			"protorev",
			"txfees",
			"incentives",
			"poolincentives",
			"tokenfactory",
			"valsetpref",
			"superfluid",
			"wasm",
			"smartaccount",
		)
		for key, value := range osmoKeys {
			keys[key] = value
		}
	}

	for _, value := range keys {
		appStore.MountStoreWithDB(value, storetypes.StoreTypeIAVL, nil)
		appStore.SetIAVLDisableFastNode(true)
	}

	err = appStore.LoadLatestVersion()
	if err != nil {
		return err
	}

	latestHeight := rootmulti.GetLatestVersion(appDB)
	if latestHeight <= 0 {
		return fmt.Errorf("the database has no valid heights to prune, the latest height: %v", latestHeight)
	}

	storeKey, exists := keys[storeName]
	if !exists {
		return fmt.Errorf("store %s does not exist", storeName)
	}

	store := appStore.GetCommitKVStore(storeKey)
	if store == nil {
		return fmt.Errorf("store %s not found", storeName)
	}

	if store.GetStoreType() != types.StoreTypeIAVL {
		return fmt.Errorf("store %s is not an IAVL store", storeName)
	}

	iavlStore := store.(*iavl.Store)
	versions := iavlStore.GetAllVersions()
	versionExists := iavlStore.VersionExists(int64(versions[0]))
	fmt.Printf("Store %s: %d versions (latest: %d, exists: %v)\n", storeName, len(versions), versions[0], versionExists)

	// Start sync pruning because of custom iavl version
	// go get github.com/osmosis-labs/iavl@7d9bfcc44282cf41bdb68a2c2c89821ee5679244
	iavlStore.DeleteVersionsTo(latestHeight - 1)

	fmt.Printf("pruning store %s complete\n", storeName)

	fmt.Println("compacting application state")
	if err := appDB.ForceCompact(nil, nil); err != nil {
		return err
	}
	fmt.Println("compacting application state complete")

	return nil
}
