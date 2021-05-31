package cmds

import (
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/castyapp/cli/hub"
	"github.com/gorilla/mux"
	"github.com/spf13/cobra"
)

type ServerCommandFlags struct {
	Host      string
	Port      int
	MediaHost string
	MediaPort int
}

func NewServerCommand() *cobra.Command {
	cFlags := new(ServerCommandFlags)
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Start a new sfu media server",
		RunE: func(cmd *cobra.Command, args []string) error {

			listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", cFlags.Host, cFlags.Port))
			if err != nil {
				log.Fatal(fmt.Errorf("Could not create ws http listener error=%v", err))
			}

			router := mux.NewRouter()

			// Listen on UDP Port 8443, will be used for all WebRTC traffic
			udpListener, err := net.ListenUDP("udp", &net.UDPAddr{
				IP:   net.ParseIP(cFlags.MediaHost),
				Port: cFlags.MediaPort,
			})
			if err != nil {
				log.Fatal(err)
			}

			svc, err := hub.NewSingPortWebrtcHub(udpListener)
			if err != nil {
				log.Fatal(err)
			}

			router.HandleFunc("/", svc.ServeHTTP)

			log.Printf("[GATEWAY] server running and listeting on ws://%s:%d", cFlags.Host, cFlags.Port)
			return fmt.Errorf("http_err: %v", http.Serve(listener, router))

		},
	}
	cmd.SuggestionsMinimumDistance = 1
	cmd.Flags().StringVarP(&cFlags.Host, "host", "H", "0.0.0.0", "Gateway host")
	cmd.Flags().StringVarP(&cFlags.MediaHost, "media-host", "M", "0.0.0.0", "Media server host")
	cmd.Flags().IntVarP(&cFlags.Port, "port", "p", 3000, "Gateway port")
	cmd.Flags().IntVarP(&cFlags.MediaPort, "media-port", "P", 62155, "Media server port")
	return cmd
}
