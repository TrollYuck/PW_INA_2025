with Ada.Text_IO; use Ada.Text_IO;
with Ada.Numerics.Float_Random; use Ada.Numerics.Float_Random;
with Random_Seeds; use Random_Seeds;
with Ada.Real_Time; use Ada.Real_Time;

procedure  Travelers2 is


-- Travelers moving on the board

  Nr_Of_Travelers : constant Integer :=15;

  Min_Steps : constant Integer := 10 ;
  Max_Steps : constant Integer := 100 ;

  Min_Delay : constant Duration := 0.01;
  Max_Delay : constant Duration := 0.05;

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
    X: Integer range 0 .. Board_Width - 1; 
    Y: Integer range 0 .. Board_Height - 1; 
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


 -- Printer Task
   task Printer is
      entry Report(Traces : Traces_Sequence_Type);
   end Printer;

   task body Printer is
   begin
      for I in 1 .. Nr_Of_Travelers loop
         accept Report(Traces : Traces_Sequence_Type) do
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
      end loop;
   end Printer;

   -- Cell Locks (Protected Objects)
   protected type Cell_Lock is
      entry Request;
      procedure Release;
      function Is_Locked return Boolean;
   private
      Locked : Boolean := False;
   end Cell_Lock;

   protected body Cell_Lock is
      entry Request when not Locked is
      begin
         Locked := True;
      end Request;

      procedure Release is
      begin
         Locked := False;
      end Release;

      function Is_Locked return Boolean is
      begin
         return Locked;
      end Is_Locked;
   end Cell_Lock;

   type Cell_Lock_Array is array (0 .. Board_Width - 1, 0 .. Board_Height - 1) of Cell_Lock;
   Cell_Locks : Cell_Lock_Array;

   type Traveler_Type is record
      Id: Integer;
      Symbol: Character;
      Position: Position_Type;    
   end record;

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
            Traveler.Position := (X => Integer(Float'Floor(Float(Board_Width) * Random(G))),
                               Y => Integer(Float'Floor(Float(Board_Height) * Random(G))));
            select
               Cell_Locks(Traveler.Position.X, Traveler.Position.Y).Request;
               exit;
            or
               delay 0.001;
            end select;
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
         begin
            Reset(G);
            case Integer(Float'Floor(4.0 * Random(G))) is
               when 0 => Move_Up(New_Pos);
               when 1 => Move_Down(New_Pos);
               when 2 => Move_Left(New_Pos);
               when 3 => Move_Right(New_Pos);
               when others => null;
            end case;

            select
               Cell_Locks(New_Pos.X, New_Pos.Y).Request;
               Cell_Locks(Traveler.Position.X, Traveler.Position.Y).Release;
               Traveler.Position := New_Pos;
               Store_Trace;
               Time_Stamp := To_Duration(Clock - Start_Time);
               
            or
               delay Max_Delay;
               Traveler.Symbol := Character'Val(Character'Pos(Traveler.Symbol) + 32);
               Store_Trace;
               Time_Stamp := To_Duration(Clock - Start_Time);
               exit;
            end select;
         end;
      end loop;
      Printer.Report(Traces);
   end Traveler_Task;

   -- Traveler Tasks Array
   Travel_Tasks : array (0 .. Nr_Of_Travelers-1) of Traveler_Task;
   Symbol       : Character := 'A';

begin
   -- Initialize Travelers
   for I in Travel_Tasks'Range loop
      Travel_Tasks(I).Init(I, Seeds(I+1), Symbol);
      Symbol := Character'Succ(Symbol);
   end loop;

   -- Start Travelers
   for I in Travel_Tasks'Range loop
      Travel_Tasks(I).Start;
   end loop;

   -- Print Board Parameters
   Put_Line("-1 " & Integer'Image(Nr_Of_Travelers) & " " &
            Integer'Image(Board_Width) & " " &
            Integer'Image(Board_Height));
end Travelers2;