/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"github.com/alist-org/alist/v3/internal/op"
	"github.com/alist-org/alist/v3/pkg/utils"
	"github.com/spf13/cobra"
)

// PasswordCmd represents the password command
var PasswordCmd = &cobra.Command{
	Use:     "admin",
	Aliases: []string{"password"},
	Short:   "Show admin user's info",
	Run: func(cmd *cobra.Command, args []string) {
		Init()
		admin, err := op.GetAdmin()
		if err != nil {
			utils.Log.Errorf("failed get admin user: %+v", err)
		} else {
			utils.Log.Infof("admin user's info: \nusername: %s\npassword: %s", admin.Username, admin.Password)
		}
	},
}

func init() {
	RootCmd.AddCommand(PasswordCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// passwordCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// passwordCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
