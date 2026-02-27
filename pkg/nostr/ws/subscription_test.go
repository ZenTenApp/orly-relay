package ws

// const RELAY = "wss://nos.lol"

// // test if we can fetch a couple of random events
// func TestSubscribeBasic(t *testing.T) {
// 	rl := mustRelayConnect(t, RELAY)
// 	defer rl.Close()
//
// 	sub, err := rl.Subscribe(
// 		context.Background(), filters.New(
// 			&filter.F{
// 				Kinds: &kinds.T{K: []*kind.T{kind.TextNote}},
// 				Limit: values.ToUintPointer(2),
// 			},
// 		),
// 	)
// 	assert.NoError(t, err)
// 	timeout := time.After(5 * time.Second)
// 	n := 0
// 	for {
// 		select {
// 		case event := <-sub.Events:
// 			assert.NotNil(t, event)
// 			n++
// 		case <-sub.EndOfStoredEvents:
// 			assert.Equal(t, 2, n)
// 			sub.Unsub()
// 			return
// 		case <-rl.Context().Done():
// 			t.Fatalf("connection closed: %v", rl.Context().Err())
// 		case <-timeout:
// 			t.Fatalf("timeout")
// 		}
// 	}
// }
//
// // test if we can do multiple nested subscriptions
// func TestNestedSubscriptions(t *testing.T) {
// 	rl := mustRelayConnect(t, RELAY)
// 	defer rl.Close()
//
// 	n := atomic.Uint32{}
//
// 	// fetch 2 replies to a note
// 	sub, err := rl.Subscribe(
// 		context.Background(), filters.New(
// 			&filter.F{
// 				Kinds: kinds.New(kind.TextNote),
// 				Tags: tags.New(
// 					tag.New(
// 						"e",
// 						"0e34a74f8547e3b95d52a2543719b109fd0312aba144e2ef95cba043f42fe8c5",
// 					),
// 				),
// 				Limit: values.ToUintPointer(3),
// 			},
// 		),
// 	)
// 	assert.NoError(t, err)
//
// 	for {
// 		select {
// 		case ev := <-sub.Events:
// 			// now fetch author of this
// 			sub, err := rl.Subscribe(
// 				context.Background(), filters.New(
// 					&filter.F{
// 						Kinds:   kinds.New(kind.ProfileMetadata),
// 						Authors: tag.New(ev.PubKeyString()),
// 						Limit:   values.ToUintPointer(1),
// 					},
// 				),
// 			)
// 			assert.NoError(t, err)
//
// 			for {
// 				select {
// 				case <-sub.Events:
// 					// do another subscription here in "sync" mode, just so
// 					// we're sure things aren't blocking
// 					rl.QuerySync(
// 						context.Background(),
// 						&filter.F{Limit: values.ToUintPointer(1)},
// 					)
// 					n.Add(1)
// 					if n.Load() == 3 {
// 						// if we get here, it means the test passed
// 						return
// 					}
// 				case <-sub.Context.Done():
// 				case <-sub.EndOfStoredEvents:
// 					sub.Unsub()
// 				}
// 			}
// 		case <-sub.EndOfStoredEvents:
// 			sub.Unsub()
// 			return
// 		case <-sub.Context.Done():
// 			t.Fatalf("connection closed: %v", rl.Context().Err())
// 			return
// 		}
// 	}
// }
