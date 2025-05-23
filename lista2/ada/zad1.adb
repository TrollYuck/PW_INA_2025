with Ada.Text_IO; use Ada.Text_IO;
with Ada.Numerics.Float_Random; use Ada.Numerics.Float_Random;
with Random_Seeds; use Random_Seeds;
with Ada.Real_Time; use Ada.Real_Time;

procedure  zad1 is


-- Travelers moving on the board

  Nr_Of_Travelers : constant Integer := 15;

  Min_Steps : constant Integer := 10 ;
  Max_Steps : constant Integer := 100 ;

  Min_Delay : constant Duration := 0.01;
  Max_Delay : constant Duration := 0.05;

  Min_Spawn_Delay : constant Duration := 0.01;
  Max_Spawn_Delay : constant Duration := 0.50;

  Min_Lifespan : constant Duration := 0.10;
  Max_Lifespan : constant Duration := 1.00;

-- 2D Board with torus topology

  Board_Width  : constant Integer := 15;
  Board_Height : constant Integer := 15;

-- Timing

  Start_Time : Time := Clock;  -- global startnig time

-- Random seeds for the tasks' random number generators
 
  Seeds : Seed_Array_Type(1..Nr_Of_Travelers) := Make_Seeds(Nr_Of_Travelers);

-- Types, procedures and functions

  -- Postitions on the board
  type Position_Type is record	
    X: Integer range 0 .. Board_Width; 
    Y: Integer range 0 .. Board_Height; 
  end record;	   

  -- elementary steps
  procedure Move_Down( Position: in out Position_Type ) is
  begin
    Position.Y := ( Position.Y + 1 ) mod Board_Height;
  end Move_Down;

  procedure Move_Up( Position: in out Position_Type ) is
  begin
    Position.Y := ( Position.Y + Board_Height - 1 ) mod Board_Height;
  end Move_Up;

  procedure Move_Right( Position: in out Position_Type ) is
  begin
    Position.X := ( Position.X + 1 ) mod Board_Width;
  end Move_Right;

  procedure Move_Left( Position: in out Position_Type ) is
  begin
    Position.X := ( Position.X + Board_Width - 1 ) mod Board_Width;
  end Move_Left;

  -- traces of travelers
  type Trace_Type is record 	      
    Time_Stamp:  Duration;	      
    Id : Integer;
    Position: Position_Type;      
    Symbol: Character;	      
  end record;	      

  type Trace_Array_type is  array(0 .. Max_Steps) of Trace_Type;

  type Traces_Sequence_Type is record
    Last: Integer := -1;
    Trace_Array: Trace_Array_type ;
  end record; 

   protected Traveler_Counter is
      procedure Increment;
      function  Value return Integer;
   private
      Count : Integer := Nr_Of_Travelers;
   end Traveler_Counter;

   protected body Traveler_Counter is
      procedure Increment is begin Count := Count + 1; end Increment;
      function Value return Integer is begin return Count; end Value;
   end Traveler_Counter;

 -- Printer Task
   task Printer is
      entry Report(Traces : Traces_Sequence_Type);
   end Printer;

   task Wild_Manager is 
      entry Start;
      entry Stop;
   end Wild_Manager;

   task body Printer is
      Reports_Received : Integer := 0;
   begin
      loop
         accept Report(Traces : Traces_Sequence_Type) do
         Reports_Received := Reports_Received + 1;
            for J in 0 .. Traces.Last loop
               declare
                  Trace : Trace_Type := Traces.Trace_Array(J);
               begin
                  Put_Line(Duration'Image(Trace.Time_Stamp) & " " &
                           Integer'Image(Trace.Id) & " " &
                           Integer'Image(Trace.Position.X) & " " &
                           Integer'Image(Trace.Position.Y) & " " &
                           Trace.Symbol);
               end;
            end loop;
         end Report;
         if Reports_Received = Nr_Of_Travelers then
            Wild_Manager.Stop;  -- signal wild manager to halt
         end if;
         exit when Reports_Received = Traveler_Counter.Value;
      end loop;
   end Printer;

   type Wild_State is (Unbothered, Pending, Moved_Out, Stayed_In);

   task type Cell_Task is
      entry Request (O: out Boolean; W: out Boolean);
      entry Occupy (W: in Boolean);
      entry Is_Waiting (W: out Boolean);
      entry Change_Wild_State (State: in Wild_State);
      entry Check_Wild_State (State: out Wild_State);
      entry Free;
   end Cell_Task;

   task body Cell_Task is
      Occupied : Boolean    := False;
      Wild     : Boolean    := False;
      Waiting  : Boolean    := False;
      W_state  : Wild_State := Unbothered;
   begin
      loop
         select
            accept Request (O: out Boolean; W: out Boolean) do
               O := not Occupied;
               W := Wild;
               if Wild then
                  Waiting := True;
               end if;
            end Request;
         or 
            accept Occupy (W: in Boolean) do
               Occupied := True;
               Wild := W;
               if Wild then
                  W_state := Unbothered;
               end if;
            end Occupy;
         or
            accept Is_Waiting (W: out Boolean) do
               W := Waiting;
            end Is_Waiting;
         or
            accept Change_Wild_State (State: in Wild_State) do
               W_state := State;
               if W_state = Moved_Out or W_state = Stayed_In then
                  Wild    := False;
                  Waiting := False;
               end if;
            end Change_Wild_State;
         or
            accept Check_Wild_State (State: out Wild_State) do
               State := W_state;
               if W_state = Moved_Out then
                  Wild := False;
               end if;
            end Check_Wild_State;
         or
            accept Free do
               Occupied := False;
               if W_state = Moved_Out then
                  Wild := False;
               end if;
            end Free;
         or 
            terminate;
         end select;
      end loop;
   end Cell_Task;

   type Cell_Task_Array is array (0 .. Board_Width - 1, 0 .. Board_Height - 1) of Cell_Task;
   Cell_Tasks: Cell_Task_Array;

   type Traveler_Type is record
      Id: Integer;
      Symbol: Character;
      Position: Position_Type;    
   end record;

   function Move(Pos : Position_Type; G : Generator) return Position_Type is
      New_Pos : Position_Type := Pos;
   begin
      case Integer(Float'Floor(4.0 * Random(G))) is
         when 0 => Move_Up(New_Pos);
         when 1 => Move_Down(New_Pos);
         when 2 => Move_Left(New_Pos);
         when 3 => Move_Right(New_Pos);
         when others => null;
      end case;
      return New_Pos;
   end Move;

   -- Traveler Task
   task type Traveler_Task is
      entry Init(Id : Integer; Seed : Integer; Symbol : Character);
      entry Start;
   end Traveler_Task;

   task body Traveler_Task is
      G           : Generator;
      Traveler    : Traveler_Type;
      Time_Stamp  : Duration;
      Steps       : Integer;
      Traces      : Traces_Sequence_Type;
      Success     : Boolean;
      Dest_Wild   : Boolean;
      Dest_State  : Wild_State;

      procedure Store_Trace is
      begin
         if Traces.Last < Max_Steps then
            Traces.Last := Traces.Last + 1;
            Traces.Trace_Array(Traces.Last) := (Time_Stamp => Time_Stamp,
                                                Id         => Traveler.Id,
                                                Position   => Traveler.Position,
                                                Symbol     => Traveler.Symbol);
         end if;
      end Store_Trace;

   begin
      accept Init(Id : Integer; Seed : Integer; Symbol : Character) do
         Traveler.Id := Id;
         Traveler.Symbol := Symbol;
         Reset(G, Seed);
         loop
            Traveler.Position := (X => Integer(Float'Floor(Float(Board_Width - 1) * Random(G))),
                               Y => Integer(Float'Floor(Float(Board_Height - 1) * Random(G))));
            Cell_Tasks(Traveler.Position.X, Traveler.Position.Y).Request(Success, Dest_Wild);
            if Success then
               Cell_Tasks(Traveler.Position.X, Traveler.Position.Y).Occupy(False); 
               exit;
            else 
               delay 0.001;
            end if;
         end loop;
         
         
         Steps := Min_Steps + Integer(Float'Floor(Float(Max_Steps - Min_Steps) * Random(G)));
         Store_Trace;
         Time_Stamp := To_Duration ( Clock - Start_Time ); -- reads global clock
      end Init;

      accept Start;

      for Step in 1 .. Steps loop
         delay Min_Delay + Duration(Float(Max_Delay - Min_Delay) * Random(G));
         declare
            New_Pos : Position_Type := Traveler.Position;
            Start_Attempt_Time : Time := Clock; -- Record the start time of the attempt
            Stuck : Boolean := False;
         begin
            New_Pos := Move (New_Pos, G);
            loop
               Cell_Tasks(New_Pos.X, New_Pos.Y).Request(Success, Dest_Wild);
               if Dest_Wild then
                  delay 0.001;
                  Cell_Tasks(New_Pos.X, New_Pos.Y).Check_Wild_State(Dest_State);
               end if;
               while Dest_State = Pending loop
                  delay 0.001;
                  Cell_Tasks(New_Pos.X, New_Pos.Y).Check_Wild_State(Dest_State);
               end loop;
               Cell_Tasks(New_Pos.X, New_Pos.Y).Request(Success, Dest_Wild);
               if Success then 
                  Cell_Tasks(Traveler.Position.X, Traveler.Position.Y).Free;
                  Cell_Tasks(New_Pos.X, New_Pos.Y).Occupy(False);
                  Traveler.Position := New_Pos;
                  exit;
               elsif Dest_State = Stayed_In then
                  New_Pos := Move (New_Pos, G);
               elsif To_Duration(Clock - Start_Attempt_Time) > Max_Delay then
                  Traveler.Symbol := Character'Val(Character'Pos(Traveler.Symbol) + 32);
                  Stuck := True;
                  exit;
               else 
                  delay 0.001;
               end if;
            end loop;
            Store_Trace;
            Time_Stamp := To_Duration(Clock - Start_Time);
            if Stuck then
               exit;
            end if;
         end;
      end loop;
      Printer.Report(Traces);
   end Traveler_Task;

   -- Traveler Tasks Array
   Travel_Tasks : array (0 .. Nr_Of_Travelers-1) of Traveler_Task;
   Symbol       : Character := 'A';

   task type Wild_Traveler_Task is
      entry Init(Id : Integer; Seed : Integer; Symbol : Character; T : Duration);
   end Wild_Traveler_Task;

   task body Wild_Traveler_Task is
      G           : Generator;
      Traveler    : Traveler_Type;
      Time_Stamp  : Duration;
      Traces      : Traces_Sequence_Type;
      Wild_Time   : Time;
      Lifespan    : Duration;
      Success     : Boolean;
      Dest_Wild   : Boolean;
      New_Pos     : Position_Type;

      procedure Store_Trace is
      begin
         if Traces.Last < Max_Steps then
            Traces.Last := Traces.Last + 1;
            Traces.Trace_Array(Traces.Last) := (Time_Stamp => Time_Stamp,
                                                Id         => Traveler.Id,
                                                Position   => Traveler.Position,
                                                Symbol     => Traveler.Symbol);
         end if;
      end Store_Trace;

      
      function Try_Relocate return Boolean is
      begin
         New_Pos := Traveler.Position;
            for J in 0 .. 3 loop
               case J is --try moving in any direction
                  when 0 => Move_Up(New_Pos);
                  when 1 => Move_Down(New_Pos);
                  when 2 => Move_Left(New_Pos);
                  when 3 => Move_Right(New_Pos);
                  when others => null;
               end case;
               Cell_Tasks(New_Pos.X, New_Pos.Y).Request(Success, Dest_Wild);
               if Success then 
                  Cell_Tasks(Traveler.Position.X, Traveler.Position.Y).Free;
                  Cell_Tasks(New_Pos.X, New_Pos.Y).Occupy(True);
                  Traveler.Position := New_Pos;
                  exit;
               end if;
            end loop;
            return Success;
      end Try_Relocate;

   begin
      accept Init(Id : Integer; Seed : Integer; Symbol : Character; T : Duration) do
         Lifespan := T;
         Traveler.Id := Id;
         Traveler.Symbol := Symbol;
         Reset(G, Seed);
         loop
            Traveler.Position := (X => Integer(Float'Floor(Float(Board_Width - 1) * Random(G))),
                               Y => Integer(Float'Floor(Float(Board_Height - 1) * Random(G))));
            Cell_Tasks(Traveler.Position.X, Traveler.Position.Y).Request(Success, Dest_Wild);
            if Success then 
               Cell_Tasks(Traveler.Position.X, Traveler.Position.Y).Occupy(True);
               exit;
            else 
               delay 0.001;
            end if;
         end loop;
         Store_Trace;
         Time_Stamp := To_Duration ( Clock - Start_Time ); -- reads global clock
         Wild_Time := Clock;
      end Init;
   
   declare
      Cell_is_waiting : Boolean;
   begin
      loop
         Cell_Tasks(Traveler.Position.X, Traveler.Position.Y).Is_Waiting(Cell_is_waiting);
         if Cell_is_waiting then
            Cell_Tasks(Traveler.Position.X, Traveler.Position.Y).Change_Wild_State(Pending);
            if Try_Relocate then
               Cell_Tasks(Traveler.Position.X, Traveler.Position.Y).Change_Wild_State(Moved_Out);
            else
               Cell_Tasks(Traveler.Position.X, Traveler.Position.Y).Change_Wild_State(Stayed_In);
            end if;
         end if;
         if To_Duration(Clock - Wild_Time) > Lifespan then
            Cell_Tasks(Traveler.Position.X, Traveler.Position.Y).Change_Wild_State(Moved_Out);
            Cell_Tasks(Traveler.Position.X, Traveler.Position.Y).Free;
            Traveler.Position := (X => Board_Width, Y => Board_Height);
            Store_Trace;
            Time_Stamp := To_Duration ( Clock - Start_Time );
            exit;
         end if;
         delay 0.001;
      end loop;
      Printer.Report(Traces);
   end;
      
   end Wild_Traveler_Task;

task body Wild_Manager is 
   Spawn_Gen   : Generator;
   Life_Gen    : Generator;
   Next_Wild_Id: Integer := Nr_Of_Travelers;
   Next_Symbol : Character := '0';
begin 
   accept Start;
   Reset(Spawn_Gen, Seeds(1));
   Reset(Life_Gen, Seeds(2));
   loop
      select
         accept Stop;
         exit;
      or
         delay Duration(Float(Max_Spawn_Delay - Min_Spawn_Delay) * Random(Spawn_Gen) + Float(Min_Spawn_Delay));
      declare
            WT : access Wild_Traveler_Task;
            Seed : Integer := Integer(Float'Floor(Float(Integer'Last) * Random(Spawn_Gen)));
            Lifespan : Duration := Min_Lifespan + Duration(Float(Max_Lifespan - Min_Lifespan) * Random(Life_Gen));
      begin
            WT := new Wild_Traveler_Task;
            WT.Init(Next_Wild_Id, Seed, Next_Symbol, Lifespan);
            Traveler_Counter.Increment;
            Next_Wild_Id := Next_Wild_Id + 1;
            if Next_Symbol = '9' then
               Next_Symbol := '0';
            else
               Next_Symbol := Character'Succ(Next_Symbol);
            end if;
      end;
      end select;
   end loop;
end Wild_Manager;


begin
   -- Initialize Travelers
   for I in Travel_Tasks'Range loop
      Travel_Tasks(I).Init(I, Seeds(I+1), Symbol);
      Symbol := Character'Succ(Symbol);
   end loop;

   Wild_Manager.Start;

   -- Start Travelers
   for I in Travel_Tasks'Range loop
      Travel_Tasks(I).Start;
   end loop;

   -- Print Board Parameters
   Put_Line("-1 " & Integer'Image(Nr_Of_Travelers) & " " &
            Integer'Image(Board_Width) & " " &
            Integer'Image(Board_Height));
end zad1;